// Package delivery renders a product's delivery_template for a subscription (customer,
// product, expiry and — for shared slots — the slot label/PIN and the master-account
// credential revealed server-side, audit-logged as delivery.render) and sends it through
// the inbox. It runs as the delivery.send job, enqueued when an invoice becomes fully paid,
// when a replacement ticket is resolved, or when an order is confirmed without a note.
package delivery
