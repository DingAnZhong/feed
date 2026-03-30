package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DingAnZhong/feed/internal/mq"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
)

func main() {
	err := config.InitConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("初始化配置失败:%v", err)
	}
	err = logger.Init(config.Conf.Log)
	if err != nil {
		log.Fatalf("初始化日志失败:%v", err)
	}
	defer logger.Sync()

	err = repository.InitDB(config.Conf.MySQL)
	if err != nil {
		log.Fatalf("初始化mysql失败:%v", err)
	}
	err = repository.InitRedis(config.Conf.Redis)
	if err != nil {
		log.Fatalf("初始化redis失败:%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = mq.StartConsumer(ctx, config.Conf.Kafka.Brokers)
	if err != nil {
		log.Fatalf("启动kafka消费者失败:%v", err)
	}
	logger.Info("🚀 Worker 消费者进程启动成功，正在监听 Kafka 消息...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("接收到退出信号，准备优雅关闭 Worker 进程...")
}
