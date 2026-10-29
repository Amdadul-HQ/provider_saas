# Lessons (append-only; newest at bottom)

Format: `## L-<n> · <date> · <area>` then What broke / Root cause / Fix / Prevention rule.
Grep this file for your area before starting a task.

## L-001 · 2026-10-29 · process
- What broke: nothing yet — seed entry.
- Root cause: —
- Fix: —
- Prevention rule: every agent logs errors here with `scripts/ai/log-lesson.sh`; never fix silently.

## L-002 · 2026-10-29 · infra
- What broke: docker run failed: daemon not running on this Mac
- Root cause: Docker Desktop is not always started; Homebrew postgresql@17 is running on :5432
- Fix: Validated migrations against local Homebrew Postgres (psql -U $USER)
- Prevention rule: Before using docker, run 'docker info' ; fall back to local Postgres (pg_isready) for DB checks; docker compose is optional in dev
