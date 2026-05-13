package models

// OrderItem is a line item on an order. ProductID is a string reference to a
// product owned by the catalog service — there is no FK across the service
// boundary.
type OrderItem struct {
	ID             string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	OrderID        string `gorm:"type:varchar(64);index;not null" json:"order_id"`
	ProductID      string `gorm:"type:varchar(64);not null" json:"product_id"`
	Qty            int32  `gorm:"not null" json:"qty"`
	UnitPriceCents int64  `gorm:"not null" json:"unit_price_cents"`
}

func (OrderItem) TableName() string { return "order_items" }
