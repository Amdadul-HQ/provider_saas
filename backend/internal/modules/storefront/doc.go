// Package storefront serves the public shop, orders and the magic-link customer portal.
// placeOrder should record a pending payment via billing.Service.RecordPayment(RecordOptions{Source: "storefront"})
// and confirmOrder should call subscription.Service.Create.
// Status: schema + stub resolvers; implementation pending (second engineer).
package storefront
