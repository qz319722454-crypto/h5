package models

import "gorm.io/gorm"

type Assignment struct {
	gorm.Model
	MiniAppID         uint `json:"MiniAppID"`
	CustomerServiceID uint `json:"CustomerServiceID"`
}
