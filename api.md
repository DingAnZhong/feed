### 🟢 全局基础规范

**1. 基础路径 (Base URL):** `/api/v1`
**2. 鉴权方式:** 所有接口（除登录注册外）均需要在 HTTP Header 中携带 JWT Token。
* `Authorization: Bearer <your_jwt_token>`
*(注：为了专注核心逻辑，这里假设网关或 Gin 中间件已经解析了 Token，并把当前用户的 `user_id` 注入到了上下文中。)*

**3. 全局统一响应结构 (Response Format):**
```json
{
  "code": 0,          // 业务状态码：0 表示成功，非 0 表示各种业务错误
  "msg": "success",   // 提示信息
  "data": {}          // 具体的业务数据，如果失败则为 null 或 {}
}
```

---

### 📝 1. 内容模块：发布帖子 (Publish Post)

这是触发我们后续 Kafka 异步写扩散（Push 模型）的源头接口。

* **接口路径:** `POST /api/v1/post/publish`
* **功能描述:** 用户发布一条新动态。
* **Content-Type:** `application/json`
* **请求参数 (Request Body):**

| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `content` | string | 是 | 帖子的文本内容（限制 500 字以内） |
| `media_urls` | array | 否 | 图片或视频的 URL 列表，最多 9 张 |

* **请求示例:**
```json
{
  "content": "今天终于把 Docker Compose 跑起来了！A级项目启动🚀",
  "media_urls": ["https://cdn.example.com/img1.jpg"]
}
```
* **成功响应 (Response):**
```json
{
  "code": 0,
  "msg": "发布成功",
  "data": {
    "post_id": "165789345210984531"  // 返回由雪花算法生成的全局唯一ID
  }
}
```
*(💡 亮点提示：此接口需做到 50ms 内返回。它只负责存 MySQL 和发 Kafka 消息，不管分发粉丝的脏活累活。)*

---

### 🤝 2. 关系模块：关注操作 (Relation Action)

决定 Feed 流推给谁的核心依据。

* **接口路径:** `POST /api/v1/relation/action`
* **功能描述:** 关注或取消关注某个用户。
* **Content-Type:** `application/json`
* **请求参数 (Request Body):**

| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `to_user_id` | int64 | 是 | 目标用户的 ID |
| `action_type` | int | 是 | 操作类型：1-关注，2-取消关注 |

* **请求示例:**
```json
{
  "to_user_id": 99887766,
  "action_type": 1
}
```
* **成功响应 (Response):**
```json
{
  "code": 0,
  "msg": "关注成功",
  "data": null
}
```

---

### 🌊 3. 核心模块：获取 Feed 流 (Get Timeline)

这是整个系统被调用最频繁、并发要求最高的接口。直接读 Redis 的收件箱！

* **接口路径:** `GET /api/v1/feed`
* **功能描述:** 下拉刷新获取自己关注的人的最新动态。
* **请求参数 (Query string):**

| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `latest_time` | int64 | 否 | **游标 (Cursor)**。如果不传，表示获取最新的；如果传了时间戳，表示获取这个时间戳之前的旧帖子。 |
| `limit` | int | 否 | 本次拉取的条数，默认 10，最大 20。 |

* **请求示例:**
`GET /api/v1/feed?latest_time=1698765432000&limit=10`

* **成功响应 (Response):**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "next_time": 1698761111000,   // 下一次请求时带上的游标（最后一条帖子的时间戳）
    "is_end": false,              // 是否已经拉到底了
    "post_list": [
      {
        "post_id": "165789345210984531",
        "author": {
          "user_id": 99887766,
          "nickname": "Go老司机",
          "avatar": "https://..."
        },
        "content": "今天终于把 Docker Compose 跑起来了！A级项目启动🚀",
        "media_urls": ["https://cdn.example.com/img1.jpg"],
        "create_time": 1698765432000, // 帖子发布时间戳
        "like_count": 42,
        "comment_count": 5
      }
      // ... 更多帖子
    ]
  }
}
```
*(💡 亮点提示：为什么不用 `page`？因为 Feed 流更新极快，用 `page` 会导致数据重复拉取或漏掉。用 `latest_time` 作为 Redis ZSet 的 Score 来做 `ZREVRANGEBYSCORE` 是标准答案！)*
