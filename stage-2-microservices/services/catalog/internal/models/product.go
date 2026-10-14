package models

import "time"

// Product is a sellable item in the catalog.
type Product struct {
	ID         string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name       string    `gorm:"type:varchar(200);not null" json:"name"`
	Category   string    `gorm:"type:varchar(100);index" json:"category"`
	PriceCents int64     `gorm:"not null" json:"price_cents"`
	StockQty   int32     `gorm:"not null;default:0" json:"stock_qty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName returns the table name for Product.
func (Product) TableName() string { return "products" }
