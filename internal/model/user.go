package model

import (
	"time"
)

// User 用户表
type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false;comment:用户ID(雪花算法)" json:"id"`
	Nickname  string    `gorm:"type:varchar(64);not null;comment:昵称" json:"nickname"`
	Avatar    string    `gorm:"type:varchar(255);default:'';comment:头像URL" json:"avatar"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// Relation 关注关系表
type Relation struct {
	ID         int64     `gorm:"primaryKey;autoIncrement;comment:物理主键" json:"-"`
	FollowerID int64     `gorm:"uniqueIndex:uk_follower_followee;not null;comment:粉丝ID" json:"follower_id"`
	FolloweeID int64     `gorm:"uniqueIndex:uk_follower_followee;index:idx_followee_id;not null;comment:大VID" json:"followee_id"`
	Status     int8      `gorm:"type:tinyint;default:1;comment:状态:1-正常,0-取消" json:"status"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Relation) TableName() string {
	return "relations"
}
