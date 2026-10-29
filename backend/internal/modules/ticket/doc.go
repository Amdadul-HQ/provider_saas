// Package ticket handles replacement/complaint tickets. resolveReplacement moves a
// subscription to a new slot (subscription.ReassignSlot + inventory picker), marks the old
// slot banned/retired and enqueues delivery.send for the new credentials.
package ticket
