package models

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	gorm.Model
	OpenID        string     `gorm:"unique"`
	MiniAppID     uint
	Subscribed    bool       // Whether user has authorized subscription messages
	LastActiveTime *time.Time `json:"LastActiveTime"` // 最后活动时间，用于判断在线状态
}
