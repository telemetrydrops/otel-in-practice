package telemetry

// Constants for telemetry

const (
	// Service scope for instrumentation
	Scope          = "telemetrydrops.com/ecommerce-monolith"
	ServiceName    = "ecommerce-monolith"
	ServiceVersion = "1.0.0"
)

// HTTP server metric (provided by net/http instrumentation; kept here for now)
const (
	HTTP_REQUEST_DURATION = "http.server.request.duration"
)

// Baggage keys for cross-cutting context propagation
const (
	BAGGAGE_PAYMENT_METHOD = "payment.method"
)

// Additional span names not in the generated registry.
const (
	// SpanEcommerceInventoryValueName is used for the aggregate inventory value query.
	SpanEcommerceInventoryValueName = "ecommerce.inventory.value"
)
