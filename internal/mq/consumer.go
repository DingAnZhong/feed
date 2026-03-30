package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// StartConsumer 启动 Kafka 消费者组
func StartConsumer(ctx context.Context, addrs []string) error {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	group, err := sarama.NewConsumerGroup(addrs, "feed_post_group", config)
	if err != nil {
		return err
	}

	handler := &PostEventConsumer{}

	go func() {
		for {
			if err := group.Consume(ctx, []string{TopicPostPublish}, handler); err != nil {
				logger.Error("Kafka 消费者组异常退出", zap.Error(err))
				return
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	return nil
}

// PostEventConsumer 实现 sarama.ConsumerGroupHandler 接口
type PostEventConsumer struct{}

func (c *PostEventConsumer) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (c *PostEventConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim 是真正干活的地方！每当有一条消息投递过来，就会进入这个循环
func (c *PostEventConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		logger.Info("Kafka 收到新发帖事件", zap.ByteString("value", msg.Value))

		var event model.PostPublishEvent
		err := json.Unmarshal(msg.Value, &event)
		if err != nil {
			logger.Error("反序列化失败，遇到毒药消息，丢弃", zap.Error(err))
			session.MarkMessage(msg, "")
			continue
		}
		followIDs, err := repository.GetFollowerIDs(context.Background(), event.UserID)
		if err != nil {
			logger.Error("获取粉丝列表失败", zap.Error(err))
			return fmt.Errorf("获取粉丝列表失败:%w", err)
		}
		if len(followIDs) > 0 {
			err = repository.PushToTimeline(context.Background(), followIDs, event.PostID, event.Timestamp)
			if err != nil {
				logger.Error("写入 Redis 收件箱失败", zap.Error(err))
				return fmt.Errorf("写入 Redis 收件箱失败:%w", err)
			}
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
