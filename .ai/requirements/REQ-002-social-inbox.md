# REQ-002 Social connect + unified inbox
Connect Facebook Page (Messenger), Instagram DM, WhatsApp Cloud API, Telegram bot. Webhooks ingest messages into `conversations/messages`. Each sender identity is linked to a customer (auto-create if new). Staff reply from one inbox; assign threads; quick-reply templates; mark order/payment from thread. Posting: create a post, pick pages, schedule.
Acceptance: webhook signature verified; token stored in vault; a new Messenger sender appears as a customer within 5s.
