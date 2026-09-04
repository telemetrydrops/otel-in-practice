package models

import (
	"time"

	"gorm.io/gorm"
)

// Product represents a product in the catalog
type Product struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description"`
	Category    string         `gorm:"index" json:"category"`
	Price       float64        `json:"price"`
	Stock       int            `json:"stock"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the table name
func (Product) TableName() string {
	return "products"
}
