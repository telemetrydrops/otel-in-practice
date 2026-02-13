package telemetry

// Constants for telemetry

const (
	// Service scope for instrumentation
	Scope          = "telemetrydrops.com/ecommerce-monolith"
	ServiceName    = "ecommerce-monolith"
	ServiceVersion = "1.0.0"
)

// Business operation spans (hierarchical naming)
const (
	SPAN_USER_REGISTRATION = "register user"
	SPAN_ORDER_PROCESSING  = "process order"
	SPAN_PRODUCT_LOOKUP    = "lookup product"
	SPAN_INVENTORY_CHECK   = "check inventory"

	// Database operations
	SPAN_USER_SELECT    = "SELECT users"
	SPAN_USER_INSERT    = "INSERT users"
	SPAN_ORDER_INSERT   = "INSERT orders"
	SPAN_PRODUCT_SELECT = "SELECT products"
	SPAN_PRODUCT_UPDATE = "UPDATE products"
)

// UCUM-compliant metrics
const (
	USER_REGISTRATIONS_TOTAL  = "users.registrations.total"
	ORDER_PROCESSING_DURATION = "orders.processing.duration"
	PRODUCT_LOOKUPS_TOTAL     = "products.lookups.total"
	HTTP_REQUEST_DURATION     = "http.server.request.duration"
)

// Baggage keys for cross-cutting context propagation
const (
	BAGGAGE_PAYMENT_METHOD = "payment.method"
)

// Business attribute constants
const (
	ATTR_USER_ID          = "user.id"
	ATTR_ORDER_ID         = "order.id"
	ATTR_PRODUCT_ID       = "product.id"
	ATTR_PRODUCT_CATEGORY = "product.category"
	ATTR_CUSTOMER_TIER    = "customer.tier"
	ATTR_PAYMENT_METHOD   = "payment.method"
	ATTR_ORDER_TOTAL      = "order.total"
)
