// Package storefront serves the public shop, orders and the magic-link customer portal.
// placeOrder records a pending payment via billing (source "storefront"); confirmOrder
// creates the subscription (slot allocation), invoices it, links + verifies the payment.
package storefront
