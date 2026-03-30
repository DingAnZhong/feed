package model

import (
	"time"
)

// Post 帖子表
type Post struct {
	ID           int64     `gorm:"primaryKey;autoIncrement:false;comment:帖子ID(雪花算法)" json:"post_id"`
	UserID       int64     `gorm:"index:idx_user_id;not null;comment:作者ID" json:"user_id"`
	Content      string    `gorm:"type:text;not null;comment:文本内容" json:"content"`
	MediaUrls    []string  `gorm:"type:json;serializer:json;comment:图片URL列表" json:"media_urls"`
	LikeCount    int       `gorm:"default:0;comment:点赞数" json:"like_count"`
	CommentCount int       `gorm:"default:0;comment:评论数" json:"comment_count"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"-"`
}

func (Post) TableName() string {
	return "posts"
}
