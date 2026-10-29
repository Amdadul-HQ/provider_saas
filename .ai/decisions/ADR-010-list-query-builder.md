# ADR-010 List query builder (pagination + search)
Status: accepted · 2026-10-29

## Context
Every repository built its list SQL by hand: an `add()` closure counting `$n`, `fmt.Sprintf`
for search `ILIKE`s, and its own keyset/LIMIT tail. Three paging styles existed
(`pagination.Page` opaque cursor, `after *uuid.UUID + limit`, plain `limit`), and resolvers
used `pageOf`, `afterID` or `pageFirst` at random. That mix caused at least one bug: the
`masterAccounts` resolver decoded `after` as an opaque cursor while the inventory table sends
the last row id, so page 2 failed with "invalid cursor".

## Decision
1. **One page type.** `pagination.Page{First, After, AfterID}` is the only paging input from
   resolver to repository. `pagination.New` accepts either the opaque cursor (Connections) or
   a row id (array lists, `arrayPageInfo` on the frontend). Resolvers always call `pageOf`.
2. **One builder.** `internal/platform/sqlb` builds the WHERE / ORDER BY / LIMIT tail:
   - `sqlb.New(tenantCol, tenantID)`: `tenantCol = $1` is always the first condition (ADR-002).
   - `Where(cond, vals...)` (each `?` binds the next value), `WhereIf`, generic `sqlb.Eq`.
   - `Search(term, fields...)` with declared fields: `Like(cols...)` (escaped `ILIKE`),
     `Exact(cols...)` (equality with the trimmed term) and `Related(from, on, fields...)`
     (`EXISTS` into another table). A blank term adds nothing.
   - `Page(page, Keyset{Sort, ID, From})`: keyset condition for either cursor form,
     `ORDER BY sort DESC, id DESC`, `LIMIT First+1`. `Limit(order, n)` for unpaged lists.
3. **Trim in the service.** Repositories return up to First+1 rows; services call
   `pagination.Trim` and either report `hasNext` (Connections) or drop it (array lists).
4. The GraphQL schema does not change; cursors stay opaque to clients.

## Consequences
- New list queries are a filter function returning `*sqlb.Query` plus a `Page` call; Count
  reuses the same filter function, so list and count can never disagree.
- `?` is the builder's placeholder; write Postgres' jsonb `?` operator via `Arg` + raw SQL.
- Audit logs keep a bigint id cursor (`id < ?`) because their ids are not UUIDs.
