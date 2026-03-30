package main

import (
	"fmt"
	"log"

	"github.com/DingAnZhong/feed/internal/api"
	"github.com/DingAnZhong/feed/internal/mq"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/DingAnZhong/feed/pkg/snowflake"
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
	err = snowflake.InitSnowflake(1)
	if err != nil {
		log.Fatalf("初始化雪花算法失败:%v", err)
	}
	err = repository.InitDB(config.Conf.MySQL)
	if err != nil {
		log.Fatalf("初始化mysql失败:%v", err)
	}
	err = repository.InitRedis(config.Conf.Redis)
	if err != nil {
		log.Fatalf("初始化redis失败:%v", err)
	}
	err = mq.InitKafka(config.Conf.Kafka.Brokers)
	if err != nil {
		log.Fatalf("初始化kakfa失败:%v", err)
	}
	r := api.SetupRouter()
	addr := fmt.Sprintf(":%d", config.Conf.App.Port)
	err = r.Run(addr)
	if err != nil {
		log.Fatalf("启动服务失败:%v", err)
	}
}
