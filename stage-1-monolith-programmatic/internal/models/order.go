package models

import (
	"time"

	"gorm.io/gorm"
)

// Order represents a customer order
type Order struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	UserID        string         `gorm:"not null;index" json:"user_id"`
	Status        string         `gorm:"default:'pending'" json:"status"` // pending, processing, completed, cancelled
	Total         float64        `json:"total"`
	PaymentMethod string         `json:"payment_method"` // credit_card, debit_card, paypal
	Items         []OrderItem    `json:"items"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	OrderID   string         `gorm:"not null;index" json:"order_id"`
	ProductID string         `gorm:"not null" json:"product_id"`
	Quantity  int            `json:"quantity"`
	Price     float64        `json:"price"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the table name for Order
func (Order) TableName() string {
	return "orders"
}

// TableName overrides the table name for OrderItem
func (OrderItem) TableName() string {
	return "order_items"
}
