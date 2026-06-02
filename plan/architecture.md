# Feed 流系统架构设计文档

## 1. 项目概述

本项目是一个面向千万级用户的社交 Feed 流系统，支持用户注册、关注关系管理、内容发布、个性化时间线推送和热门内容推荐。系统采用**推模式（Push Model）**实现 Feed 流分发，通过 Kafka 异步解耦写入与扇出，利用 Redis ZSet 实现高性能时间线缓存。

---

## 2. 系统架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Frontend (Vue 3 SPA)                         │
│                    go:embed 嵌入 Go 二进制文件                        │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP
┌──────────────────────────────▼──────────────────────────────────────┐
│                      API Server (cmd/api)                           │
│  ┌──────────┐  ┌──────────────┐  ┌────────────┐  ┌──────────────┐  │
│  │ Gin 路由  │→│   Handler    │→│  Service    │→│  Repository  │  │
│  │ + 中间件  │  │  参数校验     │  │  业务逻辑   │  │  数据访问     │  │
│  └──────────┘  └──────────────┘  └────────────┘  └──────┬───────┘  │
│                     Rate Limit                            │         │
│                     Auth (X-User-ID)                      │         │
└─────────────────────────────────────────────────────────┬──────────┘
                                                          │
              ┌───────────────────────────────────────────┼──────────┐
              │                                           │          │
              ▼                                           ▼          │
    ┌─────────────────┐                        ┌──────────────────┐  │
    │     MySQL 8.0   │                        │   Redis (ZSet)   │  │
    │  users/posts/   │                        │  feed:timeline:  │  │
    │  relations      │                        │  {userID}        │  │
    └────────┬────────┘                        └────────▲─────────┘  │
             │                                          │            │
             │         ┌─────────────────┐              │            │
             │         │  Kafka (KRaft)  │              │            │
             │         │  topic: feed    │              │            │
             │         └────────┬────────┘              │            │
             │                  │                        │            │
    ┌────────┴──────────────────▼────────────────────────┴─────────┐  │
    │              Worker (cmd/worker)                              │  │
    │  Kafka Consumer → 查询粉丝列表 → 推送到每个粉丝的 Redis Timeline │  │
    └──────────────────────────────────────────────────────────────┘  │
              └──────────────────────────────────────────────────────┘
```

---

## 3. 技术选型及依据

### 3.1 后端框架：Go + Gin

| 维度 | 说明 |
|------|------|
| **选型** | Go 1.25 + Gin v1.12 |
| **依据** | Go 的 goroutine 天然适合高并发场景，Feed 系统的扇出操作需要大量并发写入 Redis，Go 的轻量级协程模型比 Java 线程池方案开销更低。Gin 是 Go 生态中最成熟的 HTTP 框架，路由性能优异，中间件生态完善。 |
| **对比** | Java/Spring Boot：生态更成熟但内存占用大、启动慢；Node.js：单线程模型在 CPU 密集型（如 JSON 序列化大量 Feed 数据）场景下有瓶颈。 |

### 3.2 ORM：GORM

| 维度 | 说明 |
|------|------|
| **选型** | GORM v1.31 + MySQL Driver v1.6 |
| **依据** | GORM 是 Go 生态使用最广泛的 ORM，支持 AutoMigrate、关联查询、钩子函数。本项目数据模型简单（3 张表），GORM 的抽象层级合适，既避免了手写 SQL 的繁琐，又不会像 Hibernate 那样产生难以控制的 N+1 查询。 |
| **注意** | 粉丝列表查询使用了 `FIELD()` 函数保持 Redis 排序，关注操作使用了 `ON CONFLICT` 实现 upsert，这些场景 GORM 支持良好。 |

### 3.3 数据库：MySQL 8.0

| 维度 | 说明 |
|------|------|
| **选型** | MySQL 8.0 |
| **依据** | 作为关系型数据的持久层，MySQL 足以应对本场景。`users`、`posts`、`relations` 三张表之间的关系清晰，不需要复杂的图查询。MySQL 8.0 的 JSON 类型支持 `media_urls` 字段存储，窗口函数可用于后续扩展（如热门排行）。 |
| **表设计** | 主键均采用 Snowflake ID（BIGINT），避免自增 ID 在分布式环境下的冲突。`relations` 表使用 `(follower_id, followee_id)` 唯一索引防止重复关注。 |

### 3.4 缓存：Redis (ZSet)

| 维度 | 说明 |
|------|------|
| **选型** | Redis + go-redis v9 |
| **依据** | Redis 的 **Sorted Set (ZSet)** 是实现 Feed 时间线最理想的数据结构：以时间戳为 score、帖子 ID 为 member，天然支持按时间排序和分页。ZADD O(logN)、ZREVRANGE O(logN+M) 的时间复杂度完全满足实时 Feed 读取需求。 |
| **Key 设计** | `feed:timeline:{userID}` — 每个用户独立的 ZSet，上限 1000 条，通过 Lua 脚本保证原子性 ZADD + ZREMRANGEBYRANK。 |
| **分页策略** | 游标分页（基于时间戳），避免传统 offset 分页在数据量大时的性能退化。 |
| **Lua 脚本** | `push_feed.lua`：原子执行 ZADD + ZREMRANGEBYRANK，确保单次 Redis 请求完成写入和裁剪，避免并发问题。 |

### 3.5 消息队列：Kafka (KRaft 模式)

| 维度 | 说明 |
|------|------|
| **选型** | Apache Kafka (Bitnami 镜像, KRaft 模式) + IBM/Sarama v1.47 |
| **依据** | Kafka 在本系统中承担**异步解耦**的核心角色。用户发布帖子后，API Server 只需写入 MySQL + 发送 Kafka 事件即可返回，扇出操作由 Worker 异步完成。这使得发布接口延迟从 O(粉丝数) 降低到 O(1)。 |
| **为何不用 RabbitMQ** | Feed 扇出场景下消息量大（每条帖子产生一次事件，但扇出可能涉及百万级 Redis 写入），Kafka 的吞吐量和持久化能力更适合。KRaft 模式去除了 ZooKeeper 依赖，简化部署。 |
| **Consumer Group** | `feed_post_group` 保证每条消息只被一个 Worker 实例消费，支持水平扩展。 |

### 3.6 ID 生成：Snowflake

| 维度 | 说明 |
|------|------|
| **选型** | 自研 Snowflake 实现（10-bit NodeID + 12-bit Sequence） |
| **依据** | 分布式环境下需要全局唯一、趋势递增的 ID。Snowflake ID 是 BIGINT 类型可直接作为 MySQL 主键和 Redis ZSet member，比 UUID 节省存储且对 B+ 树索引友好。10-bit NodeID 支持 1024 个节点，12-bit 序列号支持每毫秒 4096 个 ID。 |
| **对比** | UUID：无序导致页分裂；数据库自增：分布式下需要发号器，成为单点瓶颈；Redis INCR：依赖 Redis 可用性。 |

### 3.7 配置管理：Viper

| 维度 | 说明 |
|------|------|
| **选型** | Viper v1.21 |
| **依据** | 支持 YAML/JSON/TOML 多格式，支持环境变量覆盖，支持配置热更新（WatchConfig）。单文件 `config.yaml` 管理所有组件配置，开发和运维简洁。 |

### 3.8 日志：Zap + Lumberjack

| 维度 | 说明 |
|------|------|
| **选型** | Zap v1.27 + Lumberjack v2.2 |
| **依据** | Zap 是 Go 生态性能最好的结构化日志库，零分配设计对高并发场景友好。Lumberjack 提供日志文件轮转，防止单个日志文件过大。dev 模式输出到控制台（带颜色），prod 模式输出 JSON 格式到文件。 |
| **GORM 集成** | 自定义 `gorm_logger.go` 桥接 Zap 到 GORM，记录慢查询（>200ms）和 SQL 错误。 |

### 3.9 中间件

| 中间件 | 实现 | 说明 |
|--------|------|------|
| **限流** | `go-redis/redis_rate` | 基于 Redis 的令牌桶限流，每 IP 10 req/s。Redis 不可用时降级放行（fail-open），保证可用性优先。 |
| **鉴权** | 自定义 Header 解析 | 读取 `X-User-ID` Header，解析为 int64 存入 Gin Context。当前为模拟鉴权，可扩展为 JWT。 |

### 3.10 前端：Vue 3 + Vite

| 维度 | 说明 |
|------|------|
| **选型** | Vue 3.5 (Composition API) + Vue Router 4.6 + Vite 8 |
| **部署方式** | `vite build` 产物通过 `go:embed` 嵌入 Go 二进制，单文件部署，无需 Nginx。 |
| **开发体验** | Vite 开发服务器通过代理转发 API 请求到 Go 后端，前端热更新独立于后端。 |

---

## 4. 核心流程

### 4.1 发布帖子（写流程）

```
Client → POST /web/api/v1/post/publish
  │
  ├─ 1. Auth 中间件解析 X-User-ID
  ├─ 2. Handler 校验参数（content 非空，≤500 字符）
  ├─ 3. Service 生成 Snowflake ID
  ├─ 4. Repository 写入 MySQL (posts 表)
  ├─ 5. Producer 发送 PostPublishEvent 到 Kafka
  └─ 6. 返回 {post_id} 给客户端（<50ms）

  --- 异步 ---

  Worker Consumer 收到事件
  ├─ 7. 查询作者粉丝列表 (GetFollowerIDs)
  ├─ 8. 16 并发 goroutine 推送到每个粉丝的 Redis ZSet
  └─ 9. Lua 脚本原子 ZADD + 裁剪（保留最近 1000 条）
```

**关键设计**：步骤 5-6 与步骤 7-9 通过 Kafka 解耦，发布接口延迟与粉丝数量无关。

### 4.2 拉取 Feed（读流程）

```
Client → GET /web/api/v1/feed/timeline?feed_type=timeline&latest_time=xxx&limit=20
  │
  ├─ 1. 从 Redis ZSet (feed:timeline:{userID}) 读取帖子 ID 列表
  │     └─ 游标分页：ZREVRANGEBYSCORE where score < latest_time
  ├─ 2. 批量查询 MySQL 获取帖子详情 (GetPostsByIDs)
  │     └─ FIELD(id, ...) 保持 Redis 排序
  ├─ 3. 组装返回（包含作者信息、时间戳）
  └─ 4. 返回 {posts[], has_more}（<20ms）
```

### 4.3 热门推荐

```
Client → GET /web/api/v1/feed/timeline?feed_type=popular&limit=20
  │
  └─ 直接查询 MySQL: ORDER BY like_count DESC LIMIT 20
```

---

## 5. 数据模型

### 5.1 ER 关系

```
users (1) ──── (N) posts        一个用户发布多篇帖子
users (N) ──── (N) users        通过 relations 表实现多对多关注
```

### 5.2 表结构

| 表 | 主键 | 核心字段 | 索引 |
|----|------|----------|------|
| `users` | BIGINT (Snowflake) | nickname, avatar | PK |
| `posts` | BIGINT (Snowflake) | user_id, content, media_urls(JSON), like_count, comment_count | PK, idx_user_id |
| `relations` | BIGINT (Auto) | follower_id, followee_id, status(1=关注/0=取消) | PK, uk_follower_followee, idx_followee_id |

---

## 6. 目录结构与分层

```
cmd/
├── api/main.go           # API Server 入口
└── worker/main.go        # Kafka Worker 入口

internal/                 # 私有业务逻辑（不可外部导入）
├── api/                  # 接口层：路由注册 + HTTP Handler
├── model/                # 模型层：GORM 实体 + 事件结构体
├── mq/                   # 消息队列层：Kafka Producer/Consumer
├── repository/           # 数据访问层：MySQL CRUD + Redis 操作
└── service/              # 业务逻辑层：校验、编排、策略

pkg/                      # 公共工具（可复用）
├── config/               # Viper 配置加载
├── logger/               # Zap 日志 + GORM 适配器
├── middleware/            # Gin 中间件（鉴权、限流）
├── response/             # 统一响应格式
└── snowflake/            # ID 生成器
```

**分层职责**：
- **Handler**：参数绑定、校验、调用 Service、返回响应
- **Service**：业务规则校验、编排多个 Repository 调用
- **Repository**：单表/单缓存的 CRUD 操作，不包含业务逻辑

---

## 7. 部署架构

### 7.1 本地开发

```bash
docker-compose up -d    # 启动 MySQL + Redis + Kafka
go run cmd/api/main.go  # 启动 API Server
go run cmd/worker/main.go  # 启动 Worker
```

### 7.2 生产部署

```
┌─────────────────────────────────────────┐
│            Load Balancer (Nginx)         │
└──────────┬──────────────┬───────────────┘
           │              │
    ┌──────▼──────┐ ┌─────▼───────┐
    │ API Server  │ │ API Server  │  ← 水平扩展
    │ Instance 1  │ │ Instance 2  │
    └──────┬──────┘ └──────┬──────┘
           │              │
    ┌──────▼──────────────▼──────┐
    │     Kafka (3 Broker)       │
    └──────┬──────────────┬──────┘
           │              │
    ┌──────▼──────┐ ┌─────▼───────┐
    │  Worker 1   │ │  Worker 2   │  ← Consumer Group 自动负载均衡
    └─────────────┘ └─────────────┘

    ┌─────────────────┐  ┌──────────────┐
    │  MySQL (主从)    │  │ Redis Cluster │
    └─────────────────┘  └──────────────┘
```

---

## 8. 性能设计要点

| 问题 | 方案 |
|------|------|
| 大 V 发帖延迟高（百万粉丝扇出） | Kafka 异步 + Worker 并发扇出，发布接口 O(1) 延迟 |
| Feed 读取性能 | Redis ZSet 直接读取，O(logN+M) 复杂度，<20ms |
| 粉丝列表过大导致 OOM | `GetFollowerIDs` 分批查询（每批 500），游标翻页 |
| Redis 写入竞争 | Lua 脚本原子 ZADD + 裁剪，避免并发问题 |
| 限流影响可用性 | Fail-open 设计：Redis 不可用时限流失效但不阻塞请求 |
| ID 冲突 | Snowflake 全局唯一，趋势递增，对索引友好 |
| 帖子详情批量查询 | MySQL `FIELD()` 函数保持 Redis 返回的排序，避免应用层二次排序 |

---

## 9. 当前局限与扩展方向

| 维度 | 现状 | 可扩展方向 |
|------|------|-----------|
| 鉴权 | 模拟鉴权（X-User-ID Header） | 接入 JWT / OAuth2 |
| 关注/粉丝数 | 无计数缓存 | **已完成** Redis 原子计数器 (`follower_counter.go`) |
| 推拉结合 | 纯推模式 | 大 V 降级为拉模式（Pull），减少扇出压力 |
| 内容审核 | 无 | 接入敏感词过滤 / 第三方审核 API |
| 单元测试 | 仅有压测工具 | **已完成** Service/Repository 层单元测试 (22 个 Service + 9 个 Repository 测试，100% 通过) |
| 监控 | 仅有日志 | Prometheus 指标 + Grafana 面板 |
| 多级缓存 | 仅 Redis | 热点用户本地缓存（BigCache / sync.Map） |

---

## 10. 压测结果与性能指标

### 10.1 测试环境

| 组件 | 版本 | 配置 |
|------|------|------|
| MySQL | 8.0 | 127.0.0.1:13306 |
| Redis | 7.0 | 127.0.0.1:6379 |
| Kafka | 3.7 | 127.0.0.1:9092 |

### 10.2 压测数据

#### 场景 1: 发帖压测 (Publish Post)
| 指标 | 数值 |
|------|------|
| 吞吐量 | 3214.44 req/s |
| 平均延迟 | 19.26ms |
| P50 延迟 | 21.88ms |
| P90 延迟 | 26.59ms |
| P95 延迟 | 27.09ms |
| P99 延迟 | 28.08ms |
| 成功率 | 100% |

#### 场景 2: Feed 流拉取 (Feed Timeline)
| 指标 | 数值 |
|------|------|
| 吞吐量 | 4155.78 req/s |
| 平均延迟 | 11.92ms |
| P50 延迟 | 9.06ms |
| P90 延迟 | 22.04ms |
| P95 延迟 | 22.54ms |
| P99 延迟 | 23.08ms |
| 成功率 | 100% |

#### 场景 3: 关注操作 (Follow/Unfollow)
| 指标 | 数值 |
|------|------|
| 吞吐量 | 7098.14 req/s |
| 平均延迟 | 7.17ms |
| P50 延迟 | 7.52ms |
| P90 延迟 | 12.61ms |
| P95 延迟 | 12.61ms |
| P99 延迟 | 13.10ms |
| 成功率 | 100% |

#### 场景 4: 混合并发压测 (Mixed Scenario)
| 指标 | 数值 |
|------|------|
| 吞吐量 | 7861.64 req/s |
| 平均延迟 | 893µs |
| 成功率 | 100% |

### 10.3 测试总结

- **Service 层**: 22 个测试全部通过
- **Repository 层**: 9 个测试全部通过
- **所有压测请求**: 100% 成功

### 10.4 性能瓶颈分析

| 瓶颈点 | 当前优化 | 进一步优化方向 |
|--------|----------|----------------|
| 发布延迟 | Kafka 异步扇出 | 热点用户降级为 Pull 模式 |
| Redis 写入 | Lua 脚本原子操作 | 批量写入 + 连接池优化 |
| 粉丝列表查询 | 分批查询 (500/批) | Redis 缓存粉丝 ID 列表 |
| 数据库连接 | 连接池 (MaxOpenConns=100) | 读写分离 + 分库分表 |

---

## 11. 单元测试详情

### 11.1 Service 层测试 (22 个)

| 测试文件 | 测试用例数 | 说明 |
|----------|-----------|------|
| `post_service_test.go` | 5 | 发帖内容校验、ID 生成、状态常量 |
| `feed_service_test.go` | 7 | Feed 拉取、分页、排序、热门推荐 |
| `user_service_test.go` | 10 | 关注、取关、用户信息、统计同步 |

### 11.2 Repository 层测试 (9 个)

| 测试文件 | 测试用例数 | 说明 |
|----------|-----------|------|
| `post_repo_test.go` | 8 | 帖子 CRUD、批量查询、状态更新 |
| `feed_cache_test.go` | 1 | 热门帖子查询 |
| `user_repo_test.go` | 0 | 关注逻辑在 Service 层测试覆盖 |

### 11.3 测试覆盖率关键点

- **Snowflake ID**: 所有测试使用雪花 ID 避免主键冲突
- **数据隔离**: 每个测试用例独立数据范围 (ID >= 3000)
- **依赖注入**: 支持 mock Redis/Kafka 进行单元测试
