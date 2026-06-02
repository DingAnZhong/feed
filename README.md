# 🚀 高并发 Feed 流系统 (Feed Stream System)

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

基于 Go 语言构建的千万级高并发 Feed 流异步分发系统。采用**推拉结合模式**架构设计，普通用户使用推模式（Push）通过 Kafka 异步扇出到粉丝 Redis Timeline；大 V 用户降级为拉模式（Pull）减少扇出压力；系统还支持拉取补充热门帖子实现内容多样性。通过 Redis Sorted Set 实现高性能时间线缓存。

## 🏗️ 架构概览 (Architecture)

本系统采用**前后端分离 + 微服务**架构：

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Vue 3 前端 SPA (go:embed)                        │
│                单文件部署，无需 Nginx，性能 optimized                 │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP
┌──────────────────────────────▼──────────────────────────────────────┐
│                   API Server (cmd/api)                              │
│  ┌──────────┐  ┌──────────────┐  ┌────────────┐  ┌──────────────┐  │
│  │ Gin 路由  │→│   Handler    │→│  Service    │→│  Repository  │  │
│  │ + 中间件  │  │  参数校验     │  │  业务逻辑   │  │  数据访问     │  │
│  └──────────┘  └──────────────┘  └────────────┘  └──────┬───────┘  │
│                     Rate Limit (Redis)                  │            │
│                     Auth (JWT)                          │            │
└─────────────────────────────────────────────────────────┼────────────┘
                                                          │
              ┌───────────────────────────────────────────┼────────────┐
              │                                           │            │
              ▼                                           ▼            │
    ┌─────────────────┐                        ┌──────────────────┐  │
    │     MySQL 8.0   │                        │   Redis (ZSet)   │  │
    │  users/posts/   │                        │  feed:timeline:  │  │
    │  relations      │                        │  {userID}        │  │
    └────────┬────────┘                        └────────▲─────────┘  │
             │                                          │            │
             │         ┌─────────────────┐              │            │
             │         │   Kafka (3.7)   │              │            │
             │         │   topic: feed   │              │            │
             │         └────────┬────────┘              │            │
             │                  │                        │            │
    ┌────────┴──────────────────▼────────────────────────┴─────────┐  │
    │              Worker (cmd/worker)                              │  │
    │  Kafka Consumer → 查询粉丝列表 → 推送到每个粉丝的 Redis Timeline │  │
    └──────────────────────────────────────────────────────────────┘  │
              └──────────────────────────────────────────────────────┘
```

* **API Node (`cmd/api`)**：提供 HTTP 接口，校验用户请求，将发帖事件投递至 Kafka 后立即返回，实现毫秒级响应。
* **Worker Node (`cmd/worker`)**：消费 Kafka 消息，执行粉丝列表查询、Redis Timeline 推送、MySQL 持久化等异步操作（仅对普通用户）。
* **核心中间件**：
  * **MySQL (8.0)**：持久化存储用户、帖子、关注关系。
  * **Redis (7.0)**：ZSet 结构实现时间线（Timeline）高速缓存。
  * **Kafka (3.7)**：KRaft 模式，实现发帖与分发的异步解耦。

### 推拉结合策略

```
用户发帖
   │
   ├─ 粉丝数 < 1000 (普通用户)
   │  └─ 推模式：Kafka 异步推送至所有粉丝 Timeline
   │
   └─ 粉丝数 ≥ 1000 (大 V)
      └─ 拉模式：仅写入作者帖子表，粉丝拉取时实时查询
```

**Feed 拉取流程**：
1. 优先从 Redis Timeline 获取（推模式帖子）
2. 若不足，补充热门帖子（拉模式）
3. 按时间倒序返回（最新在前）

---

## 🛠️ 技术栈 (Tech Stack)

| 组件 | 版本 | 用途 |
|------|------|------|
| Go | 1.21+ | 主语言 |
| Gin | 1.12 | HTTP 框架 |
| GORM | 1.31 | ORM |
| MySQL | 8.0 | 持久化存储 |
| Redis | 7.0 | 缓存 + ZSet |
| Kafka | 3.7 | 异步消息队列 |
| Vue | 3.5 | 前端框架 |
| JWT | - | 认证鉴权 |
| Viper | 1.21 | 配置管理 |
| Zap | 1.27 | 结构化日志 |
| Snowflake | - | 分布式 ID 生成 |

---

## 🛠️ 环境要求 (Prerequisites)

在运行本项目之前，请确保您的开发环境满足以下条件：
* **Golang** >= 1.20
* **Docker & Docker Compose**
* **内存 (RAM)**：>= 4GB（**重要**：由于 Kafka 与 MySQL 均为内存大户，虚拟机或物理机请务必分配至少 4GB 内存，否则可能触发 Linux OOM Killer 导致 Kafka 猝死）。

---

## 🚦 快速启动 (Quick Start)

### 1. 启动基础设施 (Infrastructure Setup)
进入项目根目录，使用 Docker Compose 一键拉起所有依赖的中间件。

```bash
docker compose up -d
```
通过 `docker ps` 确认 `feed-mysql`、`feed-redis`、`feed-kafka` 均处于 `Up` 状态且已成功映射对应端口 (3306, 6379, 9092)。

### 2. 配置文件准备 (Configuration)
确认项目根目录或 `config` 目录下的 `config.yaml` 已正确配置。示例如下：
```yaml
app:
  name: "feed-system"
  port: 8080
  env: "dev"

log:
  level: "debug"
  mode: "dev"
  filename: "./log/api.log"

mysql:
  dsn: "root:123456@tcp(127.0.0.1:13306)/feed_db?charset=utf8mb4&parseTime=True&loc=Local"
  max_open_conns: 100
  max_idle_conns: 20

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  pool_size: 100

kafka:
  brokers: 
    - "127.0.0.1:9092"
  topic_feed: "feed"
```
*(注意：请将 IP 地址替换为您实际运行 Docker 虚拟机的 IP 地址)*

### 3. 双开服务进行联调 (Run the Services)
打开两个终端，分别启动 API 节点和 Worker 节点：

**终端 A (启动 API 节点):**
```bash
go run cmd/api/main.go
```

**终端 B (启动 Worker 节点):**
```bash
go run cmd/worker/main.go
```

---

## 🧪 接口测试 (API Testing)

本系统对外暴露以下核心接口，测试时请确保 **API 节点** 和 **Worker 节点** 均已启动。

> ⚠️ 鉴权说明：所有业务接口需要 JWT Token（通过 `Authorization: Bearer <token>` Header 传递）。
> 请先调用 **注册** 或 **登录** 接口获取 Token。

### 1. 用户认证

#### 1.1 注册 / 登录 (公开接口)
```
POST /web/api/v1/user/login
```
**Body (JSON):**
```json
{
    "user_id": 1001,
    "nickname": "test_user"
}
```
**响应:**
```json
{
    "code": 0,
    "msg": "登录成功",
    "data": {
        "access_token": "eyJhbGciOiJIUzI1NiIs...",
        "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
    }
}
```

#### 1.2 刷新 Token
```
POST /web/api/v1/user/refresh
```
**Body (JSON):**
```json
{
    "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

#### 1.3 登出
```
POST /web/api/v1/user/logout
```
**Headers:** `Authorization: Bearer <token>`

---

### 2. 发布动态 (Publish Post)

发帖采用 **异步削峰** 架构。API 收到请求后，生成唯一 ID 并投递至 Kafka 立即返回，后续分发逻辑由 Worker 异步完成。

* **Method:** `POST`
* **URL:** `http://localhost:8080/web/api/v1/post/publish`
* **Headers:** * `Authorization: Bearer <token>`
* **Body (JSON):**
```json
{
    "content": "今天天气不错，系统终于跑通了！",
    "media_urls": ["https://image.com/1.jpg"]
}
```
* **业务流向**: API -> Kafka -> Worker -> (查询粉丝列表) -> (推送 Redis ZSet & 写入 MySQL)。

---

### 3. 关注用户 (Follow User)

Feed 流的核心前提是有关注关系。调用此接口建立 A 关注 B 的关系，这是 Worker 节点分发动态的依据。

* **Method:** `POST`
* **URL:** `http://localhost:8080/web/api/v1/user/follow`
* **Headers:** * `Authorization: Bearer <token>`
* **Body (JSON):**
```json
{
    "followee_id": 1001,
    "action_type": 1
}
```
> `action_type`: 1 = 关注, 2 = 取关

* **说明**: 关注成功后，当 1001 发布新动态时，当前用户的 Feed 流（Timeline）中就会出现该内容。

---

### 4. 拉取 Feed 流 (Get Feed List)

采用**推拉结合模式**。用户优先从自己的 Redis 缓存（Timeline）中拉取推模式帖子，若不足则从热门帖子池补充拉模式内容，响应速度极快。

* **Method:** `GET`
* **URL:** `http://localhost:8083/web/api/v1/feed/timeline?feed_type=timeline&latest_time=0&limit=10`
* **Headers:** `Authorization: Bearer <token>`
* **Query Params:**
    * `feed_type`: `timeline`（个人时间线）或 `popular`（热门推荐）
    * `latest_time`: `0`（分页游标，填上一页最后一条记录的时间戳）
    * `limit`: `10`（每页数量）
* **响应示例**:
```json
{
    "code": 0,
    "data": {
        "posts": [
            {
                "post_id": "1726354452100",
                "content": "今天天气不错，系统终于跑通了！",
                "author_id": 1001,
                "created_at": 1711784641
            }
        ],
        "next_time": 1711784641
    },
    "msg": "获取帖子成功"
}
```

---

### 5. 其他接口

| 接口 | Method | Path | 说明 |
|------|--------|------|------|
| 关注状态 | `GET` | `/web/api/v1/user/follow/status?followee_id=1001` | 检查当前用户是否关注了目标用户 |
| 用户信息 | `GET` | `/web/api/v1/user/info?user_id=1001` | 获取用户信息及关注/粉丝数 |
| 用户计数 | `GET` | `/web/api/v1/user/count?user_id=1001` | 独立查询关注数/粉丝数 |
| 同步计数 | `POST` | `/web/api/v1/user/sync-count?user_id=1001` | 从数据库同步关注/粉丝计数到 Redis |
| 待审核帖子 | `GET` | `/web/api/v1/admin/posts/pending?page=1&limit=20` | 获取待审核帖子列表 |
| 审核通过 | `POST` | `/web/api/v1/admin/post/approve` | 审核通过帖子 |
| 审核拒绝 | `POST` | `/web/api/v1/admin/post/reject` | 审核拒绝帖子 |

## 🏗️ 核心业务逻辑说明 (Internal Logic)

* **发帖削峰**: 发帖接口不直接操作复杂的粉丝分发，只负责把任务扔进 Kafka，保证了发帖操作的超高可用性。
* **推模式分发**: Worker 消费到发帖消息后，查询该作者的所有粉丝（MySQL），16 并发 goroutine 将帖子 ID 压入每个粉丝的 Redis ZSet 中（仅对粉丝数 < 1000 的普通用户）。
* **拉模式补充**: 大 V（粉丝数 ≥ 1000）发帖时，不推送至粉丝 Timeline，粉丝拉取时从热门帖子池实时查询补充。
* **快速分页**: Redis ZSet 使用 `ZRevRangeByScore` 时间戳作为 Score，支持极速的按时间倒序分页拉取，完美应对”刷动态”的高频场景。
* **JWT 认证**: 登录/注册后返回 access_token + refresh_token，支持无状态认证。
* **内容审核**: 帖子发布时敏感词过滤，违规帖子进入待审核状态（status=1），管理员通过 Admin 接口审核。
* **Redis 原子操作**: 使用 Lua 脚本保证 ZADD + 裁剪的原子性，确保 Timeline 不超过 1000 条上限。
* **多级缓存架构**:
  * **L1 本地缓存**：使用 `sync.Map` 实现进程内高速缓存，访问延迟 < 1ms，支持 TTL 过期机制。
  * **L2 Redis 缓存**：ZSet 结构实现时间线缓存，支持高并发读取。
  * **L3 数据库降级**：缓存未命中时查询 MySQL，查询后回填两级缓存。
  * **缓存命中率优化**：通过本地缓存减少 Redis 访问，降低 Redis 压力。
  * **循环依赖规避**：`GetUserByID`、`GetPostsByUserID`、`GetUserFollowStats` 等函数直接查询数据库，避免与缓存函数形成循环依赖。

---

## 🏗️ 多级缓存配置说明

### 缓存层级设计

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Client Request                               │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
                    ┌──────────────────┐
                    │   L1 本地缓存     │  (sync.Map, <1ms 延迟)
                    │  TTL: 可配置     │
                    └────────┬─────────┘
                             │ 命中 → 直接返回
                             │ 未命中
                             ▼
                    ┌──────────────────┐
                    │   L2 Redis 缓存   │  (ZSet/String, <5ms 延迟)
                    │  TTL: 可配置     │
                    └────────┬─────────┘
                             │ 命中 → 返回 + 回填 L1
                             │ 未命中
                             ▼
                    ┌──────────────────┐
                    │    L3 MySQL      │  (持久化存储, ~10-50ms 延迟)
                    │  回填 L1 + L2    │
                    └──────────────────┘
```

### TTL 配置

```yaml
# config.yaml 中的多级缓存配置
cache:
  local:
    default_ttl: "30s"          # 默认 TTL
    max_entries: 10000          # 最大缓存项数
    enable_cleanup: true        # 启用自动清理
    cleanup_interval: "1m"      # 清理间隔
    key_prefix: "feed:"         # 缓存键前缀
  redis:
    ttl:
      post: "60s"               # 帖子缓存 TTL
      user: "60s"               # 用户缓存 TTL
      follow_stats: "30s"       # 关注统计缓存 TTL
      popular_posts: "45s"      # 热门帖子缓存 TTL

# 推拉结合模式配置
pull_mode:
  enabled: true                 # 是否启用推拉结合模式
  huge_user_threshold: 1000     # 大 V 粉丝阈值（超过此值采用推拉结合）
  popular_post_threshold: 1000  # 热门帖子阈值（点赞数超过此值的帖子进入热门池）
```

### 缓存函数列表

| 函数名 | 描述 | 使用场景 |
|--------|------|----------|
| `CacheGetPost` | 获取帖子详情 | 帖子详情页 |
| `CacheGetUser` | 获取用户信息 | 用户主页 |
| `CacheGetFollowStats` | 获取关注/粉丝统计 | 用户信息卡片 |
| `CacheGetPopularPosts` | 获取热门帖子 | 热门推荐页 |
| `CacheGetUserPosts` | 获取用户帖子列表 | 个人主页帖子 |

### 缓存统计

```go
// 获取缓存统计信息
stats := repository.GetLocalCacheStats()
log.Printf("Local Cache Stats: hits=%d, misses=%d, invalid=%d, items=%d",
    stats.Hits, stats.Misses, stats.Invalid, stats.Items)
```

---

## 📁 目录结构 (Project Structure)

```
cmd/
├── api/main.go           # API Server 入口
└── worker/main.go        # Kafka Worker 入口

internal/                 # 私有业务逻辑
├── api/                  # Handler + 路由注册
├── model/                # GORM 实体 + 事件结构体
├── mq/                   # Kafka Producer/Consumer
├── repository/           # 数据访问层 (MySQL + Redis)
└── service/              # 业务逻辑层

pkg/                      # 公共工具
├── auth/                 # JWT 认证
├── config/               # Viper 配置加载
├── logger/               # Zap 日志 + GORM 适配器
├── middleware/            # Gin 中间件（鉴权、限流）
├── response/             # 统一响应格式
└── snowflake/            # Snowflake ID 生成器

frontend/                 # Vue 3 前端 (go:embed 嵌入)
├── src/
├── dist/                 # vite build 产物
└── ...

plan/                     # 架构设计文档
test/                     # 压测工具
```


## 🏥 常见问题排查 (Troubleshooting / FAQ)

在部署过程中如果遇到阻碍，请参考以下实战排雷经验：

1. **Kafka 容器无限重启 (Crash Loop) / 存活不足 30 秒？**
   * **原因 1：内存溢出 (OOM)**。检查 Linux 虚拟机是否只有 1-2G 内存，Kafka 在初始化时被系统强杀。请将虚拟机内存扩容至 4GB，并在 `docker-compose.yml` 中添加 `KAFKA_HEAP_OPTS: "-Xmx256m -Xms256m"`。
   * **原因 2：版本刺客与脏数据**。千万不要用 `bitnami/kafka:latest`，它的 KRaft 初始化逻辑与旧版配置不兼容。请回退至 `3.5` 版本，并执行 `docker volume prune -f` 彻底清理残留的脏数据卷后重新启动。

2. **Go 程序启动报错 `connection refused` (积极拒绝)？**
   * 检查 Linux 防火墙是否已关闭 (`systemctl stop firewalld`)。
   * 检查 `docker-compose.yml` 中是否正确配置了 `ports` 映射。
   * **注意端口映射变更**: MySQL 端口已从 `3306` 映射到 `13306`（防止与本地 MySQL 冲突），请确保 `config.yaml` 中的 `mysql.dsn` 使用 `127.0.0.1:13306`。

3. **Go 程序报错 `解析配置文件失败: %!w(<nil>)`？**
   * Viper 解析 YAML 时严格依赖结构体的 `mapstructure` 标签。请检查 `config.yaml` 中的字段名和层级是否与 Go 代码中的 `Config` 结构体**完全一致**。

4. **前端无法加载 / 404 Not Found？**
   * 确保已执行 `cd frontend && npm run build` 生成 `dist/` 目录。
   * 后端通过 `go:embed` 嵌入 `frontend-dist/` 目录，请确保构建产物已正确放置。

5. **Worker 消费 Kafka 消息失败？**
   * 确认 Kafka 容器已完全启动（`docker ps` 查看 `Up` 状态）。
   * 确认 `config.yaml` 中 `kafka.brokers` 地址与 `docker-compose.yml` 的 `KAFKA_CFG_ADVERTISED_LISTENERS` 一致。

6. **Token 刷新后旧 Token 失效？**
   * 当前实现中，每次刷新 refresh_token 会更新 Redis 中的值，旧 refresh_token 会被吊销。这是正常的。
   * access_token 有过期时间，需在前端定期刷新或在请求失败时触发重新登录。

---

## 📊 性能基准 (Performance Benchmarks)

### 压测环境

| 组件 | 版本 | 配置 |
|------|------|------|
| MySQL | 8.0 | 127.0.0.1:13306 |
| Redis | 7.0 | 127.0.0.1:6379 |
| Kafka | 3.7 | 127.0.0.1:9092 |

### 压测数据

#### 场景 1: 发帖压测 (Publish Post)

| 指标 | 数值 |
|------|------|
| 吞吐量 | 3,214.44 req/s |
| 平均延迟 | 19.26ms |
| P50 延迟 | 21.88ms |
| P95 延迟 | 27.09ms |
| P99 延迟 | 28.08ms |
| 成功率 | 100% |

#### 场景 2: Feed 流拉取 (Feed Timeline)

| 指标 | 数值 |
|------|------|
| 吞吐量 | 4,155.78 req/s |
| 平均延迟 | 11.92ms |
| P50 延迟 | 9.06ms |
| P95 延迟 | 22.54ms |
| P99 延迟 | 23.08ms |
| 成功率 | 100% |

#### 场景 3: 关注操作 (Follow/Unfollow)

| 指标 | 数值 |
|------|------|
| 吞吐量 | 7,098.14 req/s |
| 平均延迟 | 7.17ms |
| P50 延迟 | 7.52ms |
| P95 延迟 | 12.61ms |
| P99 延迟 | 13.10ms |
| 成功率 | 100% |

#### 场景 4: 混合并发压测 (Mixed Scenario)

| 指标 | 数值 |
|------|------|
| 吞吐量 | 7,861.64 req/s |
| 平均延迟 | 893µs |
| 成功率 | 100% |

---

## 🧪 测试 (Testing)

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包
go test -v ./internal/service/...
go test -v ./internal/repository/...
```

### 运行压测

```bash
# 运行全量压测
go test -bench=. -benchtime=10s ./test/...
```

### 测试覆盖

* **Service 层**: 22 个测试全部通过
* **Repository 层**: 9 个测试全部通过
* **所有压测请求**: 100% 成功

---

## 📈 性能设计要点

| 问题 | 方案 |
|------|------|
| 大 V 发帖延迟高（百万粉丝扇出） | 推拉结合：大 V 降级为拉模式，普通用户推模式，发布接口 O(1) 延迟 |
| Feed 读取性能 | Redis ZSet 直接读取，ZRevRangeByScore O(logN+M) 复杂度，<20ms |
| 粉丝列表过大导致 OOM | `GetFollowerIDs` 分批查询（每批 500），游标翻页 |
| Redis 写入竞争 | Lua 脚本原子 ZADD + 裁剪，16 并发 goroutine 推送 |
| 限流影响可用性 | Fail-open 设计：Redis 不可用时限流失效但不阻塞请求 |
| ID 冲突 | Snowflake 全局唯一，趋势递增，对索引友好 |
| 帖子详情批量查询 | MySQL `FIELD()` 函数保持 Redis 返回的排序 |
| 多级缓存 | L1 本地缓存（sync.Map） + L2 Redis + L3 DB 降级 |
| 热门补充 | Feed 拉取时自动补充热门帖子，提升内容多样性 |

---

## 🔮 未来规划 (Roadmap)

| 维度 | 当前状态 | 规划 |
|------|----------|------|
| 鉴权 | ✅ JWT + Refresh Token | OAuth2 接入 |
| 关注/粉丝数 | ✅ Redis 原子计数器 | 缓存一致性方案 |
| 内容审核 | ✅ 敏感词过滤 + 待审核队列 | AI 内容识别 |
| 监控 | ✅ 结构化日志 | Prometheus + Grafana |
| 多级缓存 | ✅ 本地缓存（sync.Map） | BigCache 性能优化 |
| 推拉结合 | ✅ 大 V 降级为拉模式（Pull） | 粉丝分层 + 混合模式 + 自动识别热点用户 |

---

## 📄 License

[MIT License](LICENSE)

---
