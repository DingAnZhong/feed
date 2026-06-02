package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// 推拉结合模式的大 V 阈值
var hugeUserFollowerThreshold int64 = 1000

func init() {
	// 从配置加载大 V 阈值
	if config.Conf.App != nil && config.Conf.App.PullMode != nil && config.Conf.App.PullMode.HugeUserThreshold > 0 {
		hugeUserFollowerThreshold = int64(config.Conf.App.PullMode.HugeUserThreshold)
	}
}

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

		// 获取作者的粉丝数，判断是否是大 V
		followerCount, _, err := repository.GetUserFollowStats(context.Background(), event.UserID)
		if err != nil {
			logger.Warn("获取粉丝数失败，使用默认策略", zap.Error(err), zap.Int64("user_id", event.UserID))
		}

		// 推拉结合策略
		if followerCount >= hugeUserFollowerThreshold {
			// 大 V 用户：推拉结合模式
			// 1. 只推送到自己的 timeline（自推）
			logger.Info("大 V 发帖，采用推拉结合模式",
				zap.Int64("user_id", event.UserID),
				zap.Int64("follower_count", followerCount),
				zap.Int64("post_id", event.PostID))

			// 推送到自己的 timeline
			err = repository.PushToSelfTimeline(context.Background(), event.UserID, event.PostID, event.Timestamp)
			if err != nil {
				logger.Error("推送到大 V 自己的 timeline 失败", zap.Error(err))
			}

			// 2. 热门帖子额外推送到热门池（供拉模式使用）
			// 这里可以添加额外逻辑：如果帖子预计会成为热门，推送到热门池
		} else {
			// 普通用户：纯推模式
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
		}

		session.MarkMessage(msg, "")
	}
	return nil
}
