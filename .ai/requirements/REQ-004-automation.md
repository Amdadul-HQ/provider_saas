# REQ-004 Automation
Reminder rules per tenant (e.g. 3 days before, on expiry, 2 days after) → jobs → messages via the customer's preferred channel. Payment inbox: SMS-forwarder endpoint posts bKash/Nagad SMS; parser extracts TrxID/amount; auto-match to pending payment → mark verified → trigger delivery template.
