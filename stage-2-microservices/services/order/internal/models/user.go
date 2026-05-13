package models

import "time"

// User is a customer.
type User struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Email     string    `gorm:"uniqueIndex;type:varchar(255);not null" json:"email"`
	Tier      string    `gorm:"type:varchar(32);not null;default:'standard'" json:"tier"`
	CreatedAt time.Time `json:"created_at"`
}

func (User) TableName() string { return "users" }
