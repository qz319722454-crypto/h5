package models

import "gorm.io/gorm"

// Config 系统配置表
type Config struct {
	gorm.Model
	Key   string `gorm:"unique" json:"Key"`   // 配置键
	Value string `json:"Value"`                // 配置值
}

