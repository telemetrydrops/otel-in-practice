package telemetry

// Baggage keys for cross-cutting context propagation. Aliased to the
// Weaver-generated attribute constants so the registry remains the single
// source of truth — if a key is renamed in attributes.yaml, callers using
// these aliases stay in sync automatically.
const (
	BaggageCustomerTier  = AttrEcommerceCustomerTier
	BaggagePaymentMethod = AttrEcommercePaymentMethod
)
