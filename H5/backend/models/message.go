package models

import "gorm.io/gorm"

type Message struct {
	gorm.Model
	UserID            uint
	CustomerServiceID uint
	Content           string
	FromUser          bool // true if from user, false if from CS
	IsImage           bool // true if message is an image
	ImageURL          string // URL of the image if IsImage is true
	IsRead            bool `gorm:"default:false"` // true if CS has read this message (from user)
	UserRead          bool `gorm:"default:false"` // true if user has read this message (from CS)
	IsDeleted         bool `gorm:"default:false"` // true if message is deleted
}
