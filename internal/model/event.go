package model

// PostPublishEvent 定义了发帖成功后投递到 Kafka 的消息结构体
// 消费者拿到这个事件后，会去查 UserID 的粉丝，并把 PostID 和 Timestamp 写入 Redis
type PostPublishEvent struct {
	PostID    int64 `json:"post_id"`
	UserID    int64 `json:"user_id"`   // 发帖人的 ID，用于后续查询他的粉丝列表
	Timestamp int64 `json:"timestamp"` // 毫秒级时间戳，用于保证写入 Redis ZSet 时的 Score 与发帖时间绝对一致
}
