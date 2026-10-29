# Goal

## One line
Give every Bangladeshi digital-subscription seller (Facebook page / WhatsApp / Telegram based) a single system that tracks every customer from every social channel and automates selling, delivering, invoicing, reminding and renewing.

## Who is the customer (tenant = "provider")
A person or small team selling Netflix / Spotify / Canva / ChatGPT / YouTube Premium / VPN / domain-hosting style digital products. They:
- get customers through Facebook page inbox, Instagram DM, WhatsApp, Telegram;
- take payment via bKash / Nagad / Rocket / bank and verify TrxIDs by hand;
- buy "master accounts" from upstream suppliers and sell slots/profiles on them;
- lose money to forgotten renewals, oversold slots, banned accounts, unpaid dues (baki);
- work 90% from a phone.

## Why they will switch to us (differentiators — build these first)
1. Unified social inbox: all channels → one inbox, each thread auto-linked to a customer record.
2. TrxID auto-verification for bKash/Nagad (SMS forwarder + manual match).
3. Slot inventory: master account → slot → customer; no overselling; upstream expiry alerts.
4. Replacement / warranty tickets.
5. Automatic reminders + magic-link customer portal + branded storefront.
6. Profit tracking (supplier cost vs selling price) and baki (due) ledger.

## Non-goals (v1)
- Not a marketplace. We do not sell accounts ourselves.
- No credential plaintext visible to platform staff. Encrypted vault only.
- No desktop-first UI. Mobile-first PWA.

## Success metric for v1
A provider can connect a Facebook page, see inbox messages become customers, create a subscription on a slot, record a bKash payment, and the customer gets an automatic expiry reminder — end to end, in under 10 minutes of onboarding.

## Legal note
Shared-account reselling violates upstream services' terms. The platform stores credentials encrypted, never views them, and the tenant accepts responsibility for their products in the ToS.
