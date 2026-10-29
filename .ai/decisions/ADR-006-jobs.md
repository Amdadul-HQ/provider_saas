# ADR-006 Background jobs
Table `jobs(kind, payload, run_at, attempts, locked_at)`; `cmd/worker` polls with `FOR UPDATE SKIP LOCKED`. Kinds: reminder.send, post.publish, social.sync, payment.match, upstream.expiry_check. Redis is not a broker; it only caches and rate-limits.
