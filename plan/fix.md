> 基于 `plan/architecture.md` 中列出的当前局限，制定本改进计划。
   > 按优先级分为 P0（核心能力）、P1（稳定性）、P2（可观测性与质量），每阶段可独立交付。

   ---

   ## 一、鉴权升级：模拟鉴权 → JWT

   ### 现状

   当前使用 `X-User-ID` 请求头传递用户 ID，无任何签名校验，客户端可伪造任意用户身份。

   ### 目标

   接入 JWT (JSON Web Token) 实现无状态鉴权，支持 Token 签发、刷新、过期吊销。

   ### 改进方案

   | 阶段 | 工作项 | 涉及文件 |
   |------|--------|---------|
   | **1. 基础鉴权** | 新增 `pkg/auth/jwt.go` — JWT 签发/验证/解析 | 新增文件 |
   | | 登录接口 `POST /web/api/v1/user/login` — 验证用户后签发 JWT | 新增 `user_handler.go` |
   | | `AuthMiddleware` 从 `Authorization: Bearer <token>` 解析 `X-User-ID` | `pkg/middleware/auth.go` |
   | | `config.yaml` 新增 `auth.jwt_secret` 和 `auth.token_ttl` | `pkg/config/config.go` |
   | **2. Token 刷新** | 新增 `POST /web/api/v1/user/refresh` — 使用 refresh_token 换取新 access_token |
   `user_handler.go` |
   | | Refresh Token 写入 Redis（TTL 7 天），支持吊销 | `repository/redis_auth.go`（新增） |
   | **3. 安全加固** | 敏感操作（关注/取关）校验 session 一致性 | `user_handler.go` |

   ### 关键设计

   ```
   ┌─────────┐  POST /login          ┌──────────┐
   │ Client  │ ───────────────────→  │  API     │
   │         │  {user_id, nickname}   │  Server  │
   └─────────┘                      └────┬─────┘
                                         │
                                         ├─ 创建/更新 user 表
                                         ├─ 生成 access_token (JWT, TTL 2h)
                                         ├─ 生成 refresh_token (Redis, TTL 7d)
                                         └─ 返回 {access_token, refresh_token}

   后续请求:
     Authorization: Bearer <access_token>
     → AuthMiddleware 解析 JWT → c.Set("userID", ...) → 正常流程
   ```

   ### 数据库改动

   无（JWT 为无状态，不依赖数据库）

   ### Redis 改动

   新增 Key：`auth:refresh_token:{user_id}` → value = `refresh_token`

   ---

   ## 二、关注/粉丝数：Redis 原子计数器

   ### 现状

   关注/粉丝数量依赖实时查询 `relations` 表，大 V 粉丝数多时查询慢。

   ### 目标

   使用 Redis INCR/DECR 原子操作维护关注数/粉丝数缓存，查询 O(1)。

   ### 改进方案

   | 阶段 | 工作项 | 涉及文件 |
   |------|--------|---------|
   | **1. 计数器实现** | 新增 `repository/follower_counter.go` — 原子递增/递减/读取 | 新增文件 |
   | | 关注操作成功后 `INCR feed:count:followees:{follower_id}` | `repository/user_repo.go` |
   | | 取关操作成功后 `DECR feed:count:followees:{follower_id}` | `repository/user_repo.go` |
   | | 同时 `INCR feed:count:followers:{followee_id}` / `DECR` | `repository/user_repo.go` |
   | **2. 查询接口** | `GET /web/api/v1/user/info?user_id=xxx` 返回 `follower_count` / `following_count` |
   `user_handler.go` |
   | | 新增 `GET /web/api/v1/user/count?user_id=xxx` 独立计数查询 | `user_handler.go` |

   ### Redis Key 设计

   | Key | 类型 | 说明 |
   |-----|------|------|
   | `feed:count:followers:{user_id}` | String (INCR) | 粉丝数 |
   | `feed:count:followees:{user_id}` | String (INCR) | 关注数 |

   ### 容错设计

   - 计数器和 DB 不一致时，提供 `GET /web/api/v1/user/sync-count` 接口，从 DB 重建 Redis 计数
   - 计数器初始化：Worker 启动时扫描 `relations` 表回填 Redis

   ---

   ## 三、推拉结合：大 V 降级为拉模式（Pull）

   ### 现状

   纯推模式，大 V（百万粉丝）发帖时 Worker 需扇出百万次 Redis 写入，延迟和成本极高。

   ### 目标

   设定大 V 阈值（如粉丝数 > 10000），超过阈值的用户发帖时：
   - **不发 Kafka 事件**（不触发扇出）
   - 仅推送到自己 timeline
   - 粉丝拉取时**合并查询**：自己的 timeline + 该作者的帖子列表

   ### 改进方案

   | 阶段 | 工作项 | 涉及文件 |
   |------|--------|---------|
   | **1. 大 V 检测** | 新增 `service/is_huge_user.go` — 从 Redis 计数器判断是否大 V | 新增文件 |
   | | 新增 `config.yaml` 配置 `app.huge_user_follower_threshold` | `pkg/config/config.go` |
   | **2. 发帖逻辑改造** | `PublishPost` 中判断是否大 V：是则仅写入 MySQL + 自己 timeline，不发送 Kafka |
   `service/post_service.go` |
   | **3. 拉取逻辑改造** | `FetchFeed` 中检测到关注关系中有大 V 时，额外查询该作者的最近 N 篇帖子 |
   `service/feed_service.go` |
   | | 新增 `repository.GetPostsByUserID()` — 查询某用户最近 N 篇帖子 | 新增 `repository/post_repo.go` |
   | **4. 合并去重排序** | 将 timeline 帖子和该作者帖子合并、去重（按 ID 去重）、按时间戳排序 |
   `service/feed_service.go` |

   ### 数据流变化

   ```
   【之前：纯推模式】
   发帖 → MySQL → Kafka → Worker 扇出到百万粉丝 → 阻塞

   【之后：推拉结合】
   用户 A（<1万粉丝）发帖 → MySQL + Kafka → Worker 扇出到粉丝
   用户 B（>1万粉丝）发帖 → MySQL + 自己 timeline → 粉丝拉取时合并查询
   ```

   ### 拉取合并伪代码

   ```go
   func FetchFeed(ctx, userID, latestTime, limit) (posts, nextTime, err) {
       // 1. 从 Redis 拉取时间线帖子 ID
       timelineIDs := GetTimeline(ctx, userID, latestTime, limit)

       // 2. 查询用户关注的大 V 列表（粉丝数 > 阈值）
       hugeFollowings := GetHugeFollowings(ctx, userID)

       // 3. 对每个大 V，查最近 N 篇帖子
       hugePostIDs := []
       for _, hugeID := range hugeFollowings {
           ids := GetRecentPostIDsByUser(ctx, hugeID, 50)
           hugePostIDs = append(hugePostIDs, ids...)
       }

       // 4. 合并 timeline + hugePostIDs，去重、排序、裁剪到 limit
       merged := MergeAndDedup(timelineIDs, hugePostIDs, limit)

       return GetPostsByIDs(ctx, merged)
   }
   ```

   ---

   ## 四、内容审核：敏感词过滤

   ### 现状

   帖子内容无任何审核，可发布违规内容。

   ### 目标

   发帖时异步检测敏感词，标记违规内容并通知管理员。

   ### 改进方案

   | 阶段 | 工作项 | 涉及文件 |
   |------|--------|---------|
   | **1. 敏感词库** | 新增 `internal/filter/dict.go` — 加载敏感词列表（Trie 树实现） | 新增文件 |
   | | 支持热更新：监听文件变更或从 Redis 加载 | `dict.go` |
   | **2. 发帖审核** | `PublishPost` 中调用敏感词检测，命中则标记 `post.status = 2`（待审核） |
   `service/post_service.go` |
   | | 审核未通过的帖子不推送到时间线 | `service/feed_service.go` |
   | **3. 审核后台** | 新增 `GET /web/api/v1/admin/posts/pending` — 待审核列表 | 新增 handler |
   | | 新增 `POST /web/api/v1/admin/post/approve` / `reject` | 新增 handler |

   ### 审核状态设计

   在 `posts` 表新增 `status` 字段：

   | status | 含义 |
   |--------|------|
   | 0 | 正常 |
   | 1 | 审核中 |
   | 2 | 审核不通过 |

   ```sql
   ALTER TABLE posts ADD COLUMN status TINYINT DEFAULT 0 COMMENT '0-正常 1-审核中 2-不通过';
   ```

   ### Trie 树实现要点

   ```
   词库: ["违法", "赌博", "诈骗"]
   构建 Trie → 发帖内容逐字符匹配 → 命中词标记审核
   ```

   ---

   ## 五、单元测试

   ### 现状

   仅有压测工具，无 Service/Repository 层单元测试。

   ### 目标

   核心业务逻辑覆盖率达 80%+，CI 可自动运行。

   ### 改进方案

   | 优先级 | 测试范围 | 覆盖点 |
   |--------|---------|--------|
   | **P0** | `service/post_service.go` | 内容校验（空内容、超长）、雪花 ID 生成、并发安全 |
   | **P0** | `service/feed_service.go` | 游标分页正确性、热门排序、空结果处理 |
   | **P1** | `service/user_service.go` | 关注/取关、不能关注自己、用户不存在 |
   | **P1** | `repository/post_repo.go` | FIELD 排序、批量查询、IN 查询边界 |
   | **P1** | `repository/feed_cache.go` | Lua 脚本逻辑、游标分页、ZSet 操作 |
   | **P2** | `pkg/middleware/ratelimit.go` | 限流触发、降级放行 |

   ### 测试策略

   | 场景 | 方式 | 说明 |
   |------|------|------|
   | Service 层单元测试 | `testing` + 接口 mock | 通过 Go interface mock 隔离 Repository |
   | Repository 层集成测试 | `testcontainers-go` | 启动临时 MySQL + Redis 容器 |
   | API 层端到端测试 | `httptest` + `gin/test` | 模拟 HTTP 请求，验证响应码和结构 |

   ### 关键代码改动

   ```go
   // 1. Repository 层改为接口，方便 mock
   type PostRepository interface {
       CreatePost(ctx context.Context, post *model.Post) error
       GetPostsByIDs(ctx context.Context, ids []int64) ([]*model.Post, error)
   }

   // 2. Service 依赖注入接口而非直接调用包级函数
   type PostService struct {
       repo PostRepository
   }

   func NewPostService(repo PostRepository) *PostService {
       return &PostService{repo: repo}
   }

   // 3. 测试文件
   // internal/service/post_service_test.go
   // internal/service/feed_service_test.go
   // internal/repository/post_repo_test.go
   // internal/repository/feed_cache_test.go
   ```

   ---

   ## 六、Prometheus 监控

   ### 现状

   仅有日志，无结构化指标，无法做告警和性能分析。

   ### 目标

   暴露 Prometheus 指标，配套 Grafana 面板，关键指标设置告警阈值。

   ### 改进方案

   | 阶段 | 工作项 | 涉及文件 |
   |------|--------|---------|
   | **1. 指标定义** | 新增 `pkg/metrics/metrics.go` — 定义业务指标 | 新增文件 |
   | | API 请求延迟（Histogram）：`api_request_duration_seconds` | `middleware/metrics.go` |
   | | API 请求计数（Counter）：`api_request_total{method,endpoint,status}` | `middleware/metrics.go` |
   | | Kafka 消息发送延迟（Histogram） | `mq/producer.go` |
   | | Redis 操作计数（Counter） | `repository/redis.go` |
   | **2. 中间件集成** | 新增 `pkg/middleware/metrics.go` — Prometheus 中间件 | 新增文件 |
   | | 注册到 Gin 路由 | `api/router.go` |
   | **3. 端点暴露** | 新增 `GET /metrics` 端点 | `api/router.go` |
   | **4. 告警规则** | 定义 AlertManager 规则 | 新增 `alerting/rules.yml` |

   ### 核心指标清单

   | 指标名称 | 类型 | 标签 | 说明 |
   |---------|------|------|------|
   | `api_request_duration_seconds` | Histogram | `method`, `endpoint`, `status` | HTTP 请求延迟 |
   | `api_request_total` | Counter | `method`, `endpoint`, `status` | HTTP 请求总数 |
   | `kafka_publish_duration_seconds` | Histogram | 无 | Kafka 消息发送延迟 |
   | `kafka_publish_total` | Counter | `topic`, `success` | Kafka 消息发送次数 |
   | `feed_push_duration_seconds` | Histogram | 无 | Feed 扇出耗时 |
   | `follower_count_gauge` | Gauge | `user_id` | 用户粉丝数（来自 Redis 计数器） |
   | `post_create_total` | Counter | 无 | 发帖总数 |
   | `post_create_duration_seconds` | Histogram | 无 | 发帖耗时 |

   ### 告警规则示例

   ```yaml
   groups:
     - name: feed-alerts
       rules:
         - alert: HighErrorRate
           expr: rate(api_request_total{status=~"5.."}[5m]) / rate(api_request_total[5m]) > 0.05
           for: 2m
           labels: { severity: critical }
           annotations: { summary: "5xx 错误率超过 5%" }

         - alert: HighLatency
           expr: histogram_quantile(0.99, rate(api_request_duration_seconds_bucket[5m])) > 0.5
           for: 5m
           labels: { severity: warning }
           annotations: { summary: "P99 延迟超过 500ms" }

         - alert: KafkaConsumerLag
           expr: kafka_consumer_group_lag > 10000
           for: 10m
           labels: { severity: warning }
           annotations: { summary: "Kafka 消费者积压超过 10000" }
   ```

   ---

   ## 七、多级缓存：热点用户本地缓存

   ### 现状

   仅依赖 Redis 缓存，高频请求仍有网络开销，且 Redis 单点故障影响全局。

   ### 目标

   对热点数据（如用户信息、帖子详情）引入本地缓存层，降低 Redis 压力。

   ### 改进方案

   | 阶段 | 工作项 | 涉及文件 |
   |------|--------|---------|
   | **1. 缓存策略设计** | 新增 `pkg/cache/layered.go` — 两级缓存管理 | 新增文件 |
   | | 本地缓存：`sync.Map` 或 `bigcache`，TTL 30s | 新增文件 |
   | | 缓存穿透防护：空值缓存（TTL 5s） | `layered.go` |
   | **2. 用户信息缓存** | `GetUserInfo` 先查本地缓存 → 未命中查 Redis → 回写本地 | `service/user_service.go` |
   | **3. 帖子详情缓存** | `GetPostsByIDs` 先查本地缓存 → 未命中查 MySQL → 回写本地 | `repository/post_repo.go` |
   | **4. 缓存失效** | 发帖成功后清除用户 timeline 相关缓存 | `service/post_service.go` |

   ### 两级缓存架构

   ```
   请求 → 本地缓存 (sync.Map, TTL 30s)
            │ 命中 → 返回
            │ 未命中
            ↓
        Redis (TTL 60s)
            │ 命中 → 回写本地缓存 → 返回
            │ 未命中
            ↓
        MySQL
            → 回写 Redis + 本地缓存
   ```

   ### 本地缓存配置

   ```yaml
   app:
     local_cache:
       enabled: true
       ttl_seconds: 30
       max_items: 10000
       empty_cache_ttl_seconds: 5
   ```

   ---

   ## 八、改进全景图

   ### 改进前后对比

   | 维度 | 改进前 | 改进后 |
   |------|--------|--------|
   | **鉴权** | X-User-ID 明文传递，可伪造 | JWT + Refresh Token，安全无状态 |
   | **关注计数** | 实时查 DB，O(N) | Redis INCR 原子计数，O(1) |
   | **大 V 扇出** | 纯推，百万粉丝阻塞 | 推拉结合，大 V 降级为拉模式 |
   | **内容安全** | 无审核 | Trie 树敏感词过滤 + 审核后台 |
   | **代码质量** | 仅压测工具 | Service/Repository 层覆盖 80%+ 单元测试 |
   | **可观测性** | 仅日志 | Prometheus + Grafana + AlertManager |
   | **缓存层次** | 仅 Redis | Redis + 本地缓存两级架构 |

   ### 实施路线图

   ```
   阶段一 (P0) ── 2 周 ──────────────────────────────────
     ├─ JWT 鉴权（含登录、刷新）
     ├─ Redis 关注/粉丝计数器
     └─ 内容审核（敏感词过滤 + 审核后台）

   阶段二 (P1) ── 3 周 ──────────────────────────────────
     ├─ 推拉结合（大 V 检测 + 扇出降级）
     ├─ Prometheus 监控（指标定义 + 中间件 + 告警规则）
     └─ 核心层单元测试（Service + Repository）

   阶段三 (P2) ── 2 周 ──────────────────────────────────
     ├─ 多级本地缓存（两级缓存 + 穿透防护）
     ├─ 补充 API 层端到端测试
     └─ CI 流水线集成（lint + test + build）
   ```

   ---

   ## 九、配置文件变更汇总

   新增/修改的 `config/config.yaml` 内容：

   ```yaml
   app:
     name: "feed-api"
     port: 8080
     env: "dev"
     # 新增：大 V 阈值（粉丝数超过此值降级为拉模式）
     huge_user_follower_threshold: 10000
     # 新增：本地缓存配置
     local_cache:
       enabled: true
       ttl_seconds: 30
       max_items: 10000
       empty_cache_ttl_seconds: 5

   # 新增：JWT 鉴权配置
   auth:
     jwt_secret: "your-secret-key-change-in-production"
     token_ttl: "2h"
     refresh_token_ttl: "720h"  # 7天

   mysql:
     dsn: "user:pass@tcp(127.0.0.1:3306)/feed?charset=utf8mb4&parseTime=True&loc=Local"
     max_open_conns: 100
     max_idle_conns: 10

   redis:
     addr: "127.0.0.1:6379"
     password: ""
     db: 0
     pool_size: 20

   kafka:
     brokers:
       - "127.0.0.1:9092"
     topic_feed: "feed"

   log:
     level: "info"
     mode: "dev"
     filename: "log/app.log"
     max_size: 100
     max_backups: 10
     max_age: 7
   ```

   ---

   ## 十、数据库迁移 SQL

   ```sql
   -- 1. posts 表新增 status 字段（内容审核）
   ALTER TABLE posts
   ADD COLUMN status TINYINT DEFAULT 0 COMMENT '0-正常 1-审核中 2-不通过'
   AFTER media_urls;

   -- 2. 初始化已有帖子状态
   UPDATE posts SET status = 0 WHERE status IS NULL;
   ```

   ---

   ## 附录：文件新增/修改清单

   | 文件路径 | 操作 | 说明 |
   |---------|------|------|
   | `pkg/auth/jwt.go` | **新增** | JWT 签发、验证、解析 |
   | `pkg/middleware/metrics.go` | **新增** | Prometheus 中间件 |
   | `pkg/cache/layered.go` | **新增** | 两级缓存管理 |
   | `internal/filter/dict.go` | **新增** | 敏感词字典 + Trie 树 |
   | `internal/service/is_huge_user.go` | **新增** | 大 V 检测逻辑 |
   | `internal/repository/follower_counter.go` | **新增** | Redis 原子计数器 |
   | `internal/repository/post_repo.go` | 修改 | 新增 `GetPostsByUserID`，支持本地缓存 |
   | `internal/service/post_service.go` | 修改 | 敏感词检测、大 V 判断 |
   ## 附录：文件新增/修改清单

   | 文件路径 | 操作 | 说明 |
   |---------|------|------|
   | `pkg/auth/jwt.go` | **新增** | JWT 签发、验证、解析 |
   | `pkg/middleware/metrics.go` | **新增** | Prometheus 中间件 |
   | `pkg/cache/layered.go` | **新增** | 两级缓存管理 |
   | `internal/filter/dict.go` | **新增** | 敏感词字典 + Trie 树 |
   | `internal/service/is_huge_user.go` | **新增** | 大 V 检测逻辑 |
   | `internal/repository/follower_counter.go` | **新增** | Redis 原子计数器 |
   | `internal/repository/post_repo.go` | 修改 | 新增 `GetPostsByUserID`，支持本地缓存 |
   | `internal/service/post_service.go` | 修改 | 敏感词检测、大 V 判断 |

   ---
  🎯 建议的下一步计划

  基于当前代码状态和改进计划的优先级，我建议按以下顺序推进：

  Step 1：配置文件升级（前置工作） ⬅️  应该立即做 ✅ 已完成

  当前 pkg/config/config.go 缺少新增配置项，这是后续所有改进的基础：
  - 新增 AuthConfig（jwt_secret、token_ttl、refresh_token_ttl）
  - 新增 AppConfig 扩展字段（huge_user_follower_threshold、local_cache）
  - 同时更新 config.yaml

  Step 2：数据库迁移 SQL ⬬ 前置依赖 ✅ 已完成

  fix.md 第十条定义的 posts 表 status 字段迁移需要先行执行：
  ALTER TABLE posts ADD COLUMN status TINYINT DEFAULT 0 COMMENT '0-正常 1-审核中 2-不通过';

  Step 3：JWT 鉴权系统（P0 核心）✅ 已完成

  这是阶段一的首要任务，涉及面较广：
  1. 新增 pkg/auth/jwt.go — JWT 签发/验证/解析
  2. 新增 pkg/middleware/auth.go — 替换当前 X-User-ID 模拟鉴权为 JWT Bearer Token
  3. 改造 user_handler.go — 新增 LoginHandler、RefreshHandler
  4. 新增 internal/repository/redis_auth.go — Refresh Token 的 Redis 存取与吊销
  5. 改造现有 Handler — 所有需要鉴权的接口自动走 JWT 解析

  Step 4：Redis 关注/粉丝原子计数器（P0）✅ 已完成

  1. 新增 internal/repository/follower_counter.go — INCR/DECR 原子操作封装
  2. 改造 user_repo.go — 关注/取关操作后同步更新 Redis 计数器
  3. 改造 user_handler.go — /user/info 返回 follower_count / following_count
  4. 新增计数同步接口 — /user/sync-count 用于数据一致性修复

  Step 5：内容审核 — 敏感词过滤（P0）✅ 已完成

  1. 新增 internal/filter/dict.go — Trie 树敏感词库
  2. 数据库迁移执行 — posts 表加 status 字段
  3. 改造 post_service.go — 发帖时敏感词检测，命中则标记审核
  4. 新增 admin handler — 待审核帖子列表 + 审核通过/拒绝

