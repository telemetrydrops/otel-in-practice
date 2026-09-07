package models

import "time"

// Order represents a customer purchase.
type Order struct {
	ID            string      `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID        string      `gorm:"type:varchar(64);index;not null" json:"user_id"`
	Status        string      `gorm:"type:varchar(32);not null" json:"status"`
	TotalCents    int64       `gorm:"not null" json:"total_cents"`
	PaymentMethod string      `gorm:"type:varchar(32)" json:"payment_method"`
	Items         []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE" json:"items"`
	CreatedAt     time.Time   `json:"created_at"`
}

func (Order) TableName() string { return "orders" }
