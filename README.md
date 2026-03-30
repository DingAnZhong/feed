# 🚀 高并发 Feed 流系统 (Feed Stream System)

基于 Go 语言构建的千万级高并发 Feed 流异步分发系统。采用“推拉结合”的架构设计，通过 Kafka 实现核心业务的异步削峰，并结合 Redis 缓存与 MySQL 持久化，保障系统的极速响应与高可用。

## 🏗️ 架构概览 (Architecture)

本系统拆分为**前台接客（API 节点）**与**后台干活（Worker 节点）**两个独立的服务，完美实现业务解耦。

* **API Node (`cmd/api`)**：负责提供 HTTP 接口，极速校验用户请求，将发帖事件投递至 Kafka 后立即返回 200 OK，实现毫秒级响应。
* **Worker Node (`cmd/worker`)**：在后台默默消费 Kafka 消息，执行“查粉丝列表”、“推送 Redis 缓存 (Timeline)”、“落盘 MySQL”等重体力劳动。
* **核心中间件**：
  * **MySQL (8.0)**：负责元数据（用户、帖子详情）的持久化存储。
  * **Redis**：采用 ZSet 结构实现时间线（Timeline）的高速缓存，支持快速分页拉取。
  * **Kafka**：采用 KRaft 模式，作为千万级并发的“大动脉”，彻底解耦发帖与分发流程。

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
  dsn: "root:123456@tcp(127.0.0.1:3306)/feed_db?charset=utf8mb4&parseTime=True&loc=Local"
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

本系统对外暴露三个核心接口，测试时请确保 **API 节点** 和 **Worker 节点** 均已启动。

### 1. 发布动态 (Publish Post)
这是系统的入口，采用 **异步削峰** 架构。API 收到请求后，生成唯一 ID 并投递至 Kafka 立即返回，后续分发逻辑由 Worker 异步完成。

* **Method:** `POST`
* **URL:** `http://localhost:8080/api/v1/post/publish`
* **Headers:** * `X-User-ID: 1001` (模拟发送者 ID)
* **Body (JSON):**
```json
{
    "content": "今天天气不错，系统终于跑通了！",
    "media_urls": ["https://image.com/1.jpg"]
}
```
* **业务流向**: API -> Kafka -> Worker -> (查询粉丝列表) -> (推送 Redis ZSet & 写入 MySQL)。

---

### 2. 关注用户 (Follow User)
Feed 流的核心前提是有关注关系。调用此接口建立 A 关注 B 的关系，这是 Worker 节点分发动态的依据。

* **Method:** `POST`
* **URL:** `http://localhost:8080/api/v1/user/follow`
* **Headers:** * `X-User-ID: 2002` (模拟粉丝 ID，即当前操作用户)
* **Body (JSON):**
```json
{
    "target_user_id": 1001
}
```
* **说明**: 关注成功后，当 1001 发布新动态时，2002 的 Feed 流（Timeline）中就会出现该内容。

---

### 3. 拉取 Feed 流 (Get Feed List)
采用 **推模式 (Push Model)**。用户直接从自己的 Redis 缓存（Timeline）中拉取已经分发好的动态 ID 列表，响应速度极快。

* **Method:** `GET`
* **URL:** `http://localhost:8080/api/v1/feed/list`
* **Headers:** * `X-User-ID: 2002` (模拟拉取者 ID)
* **Query Params:**
    * `last_id`: `0` (分页游标，填上一页最后一条记录的 ID)
    * `size`: `10` (每页数量)
* **响应示例**:
```json
{
    "code": 200,
    "data": [
        {
            "post_id": "1726354452100",
            "content": "今天天气不错，系统终于跑通了！",
            "author_id": 1001,
            "created_at": 1711784641
        }
    ],
    "msg": "success"
}
```

---

## 🏗️ 核心业务逻辑说明 (Internal Logic)

* **发帖削峰**: 发帖接口不直接操作复杂的粉丝分发，只负责把任务扔进 Kafka，保证了发帖操作的超高可用性。
* **推模式分发**: Worker 消费到发帖消息后，实时查询该作者的所有粉丝（MySQL），并循环将帖子 ID 压入每个粉丝的 Redis ZSet 中。
* **快速分页**: Redis ZSet 使用时间戳作为 Score，支持极速的按时间倒序分页拉取，完美应对“刷动态”的高频场景。

---


## 🏥 常见问题排查 (Troubleshooting / FAQ)

在部署过程中如果遇到阻碍，请参考以下实战排雷经验：

1. **Kafka 容器无限重启 (Crash Loop) / 存活不足 30 秒？**
   * **原因 1：内存溢出 (OOM)**。检查 Linux 虚拟机是否只有 1-2G 内存，Kafka 在初始化时被系统强杀。请将虚拟机内存扩容至 4GB，并在 `docker-compose.yml` 中添加 `KAFKA_HEAP_OPTS: "-Xmx256m -Xms256m"`。
   * **原因 2：版本刺客与脏数据**。千万不要用 `bitnami/kafka:latest`，它的 KRaft 初始化逻辑与旧版配置不兼容。请回退至 `3.5` 版本，并执行 `docker volume prune -f` 彻底清理残留的脏数据卷后重新启动。

2. **Go 程序启动报错 `connection refused` (积极拒绝)？**
   * 检查 Linux 防火墙是否已关闭 (`systemctl stop firewalld`)。
   * 检查 `docker-compose.yml` 中是否正确配置了 `ports` 映射（如 `"3306:3306"`）。如果没有映射，容器只在 Docker 内部网络通信，外部无法访问。

3. **Go 程序报错 `解析配置文件失败: %!w(<nil>)`？**
   * Viper 解析 YAML 时严格依赖结构体的 `mapstructure` 标签。请检查 `config.yaml` 中的字段名和层级是否与 Go 代码中的 `Config` 结构体**完全一致**（例如 `topic_feed` 不要错写成 `topic_seckill`）。

---
*Built with ❤️ and a lot of debugging.*
