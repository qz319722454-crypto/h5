package models

import "gorm.io/gorm"

type CustomerService struct {
	gorm.Model
	Name          string `gorm:"unique" json:"Name"`
	Password      string `json:"-"` // Hashed password for auth, don't return in JSON
	IsAdmin       bool   `json:"IsAdmin"` // true if admin, false if customer service
	QRCodePath    string `json:"QRCodePath"` // 小程序二维码路径，用于生成二维码
	WelcomeMessage string `json:"WelcomeMessage"` // 欢迎语，用户首次发送消息时自动发送
}
