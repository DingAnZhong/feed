package mq

import (
	"encoding/json"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/IBM/sarama"
)

// 定义 Topic 常量
const TopicPostPublish = "feed"

// 全局 Kafka 生产者实例
var Producer sarama.SyncProducer

// InitKafka 初始化 Kafka 生产者
func InitKafka(addrs []string) error {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	var err error
	Producer, err = sarama.NewSyncProducer(addrs, config)
	if err != nil {
		return fmt.Errorf("创建 SyncProducer 失败:%w", err)
	}
	return nil
}

// SendPostPublishEvent 发送发帖事件到 Kafka
func SendPostPublishEvent(event *model.PostPublishEvent) error {
	// 如果 Kafka 未初始化，返回空（跳过消息发送）
	if Producer == nil {
		return nil
	}
	bytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("消息序列化失败：%w", err)
	}
	msg := &sarama.ProducerMessage{
		Topic: TopicPostPublish,
		Value: sarama.ByteEncoder(bytes),
	}

	_, _, err = Producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("发送消息失败：%w", err)
	}
	return nil
}
