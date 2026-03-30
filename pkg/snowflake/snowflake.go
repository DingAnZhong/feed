package snowflake

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 定义雪花算法相关的常量
const (
	epoch        int64 = 1711238400000 // 起始时间戳
	nodeBits     uint8 = 10            // 机器 ID 所占的位数
	sequenceBits uint8 = 12            // 序列号所占的位数

	maxNodeID   int64 = -1 ^ (-1 << nodeBits)     // 节点 ID 的最大值 (1023)
	maxSequence int64 = -1 ^ (-1 << sequenceBits) // 序列号的最大值 (4095)

	nodeShift      uint8 = sequenceBits            // 节点 ID 向左移 12 位
	timestampShift uint8 = sequenceBits + nodeBits // 时间戳向左移 22 位 (12+10)
)

// Snowflake 节点结构体
type Snowflake struct {
	mutex     sync.Mutex // 并发安全锁
	timestamp int64      // 上次生成 ID 的时间戳
	nodeID    int64      // 当前节点的机器 ID
	sequence  int64      // 当前毫秒内的序列号
}

// 全局单例
var node *Snowflake

// InitSnowflake 初始化雪花算法节点 (在 main.go 中调用)
// 参数:
//
//	machineID: 当前机器的编号 (0 ~ 1023)
func InitSnowflake(machineID int64) error {
	if machineID < 0 || machineID > maxNodeID {
		return errors.New("节点 ID 超出范围")
	}
	node = &Snowflake{
		timestamp: 0,
		nodeID:    machineID,
		sequence:  0,
	}
	return nil
}

// GenerateID 生成一个全局唯一的 int64 ID
func GenerateID() (int64, error) {
	if node == nil {
		return 0, errors.New("雪花算法未初始化")
	}
	return node.generate()
}

func (s *Snowflake) generate() (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now().UnixNano() / 1e6

	if now < s.timestamp {
		return -1, fmt.Errorf("严重错误：出现时钟回拨，拒绝生成 ID")
	}

	if now == s.timestamp {
		// 与运算保证了超过4095则减掉4095，结果是到4096就归0进入if分支
		s.sequence = (s.sequence + 1) & maxSequence
		// 如果这一毫秒的序列号用完了 (变成了0)
		if s.sequence == 0 {
			// 死循环等待，直到系统时间跳到下一毫秒
			for now <= s.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		s.sequence = 0
	}

	s.timestamp = now

	res := ((now - epoch) << timestampShift) | (s.nodeID << nodeShift) | s.sequence

	return res, nil
}
