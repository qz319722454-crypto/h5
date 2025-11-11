package models

import "gorm.io/gorm"

type MiniApp struct {
	gorm.Model
	Name       string `json:"Name"`       // 小程序名称
	AppID      string `gorm:"unique" json:"AppID"`
	Secret     string `json:"Secret"`     // Optional, for API access if needed
	TemplateID string `json:"TemplateID"` // WeChat subscription message template ID
}
