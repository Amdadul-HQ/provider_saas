# ADR-002 Multi-tenancy
**Decision**: single database, every business table has `tenant_id uuid not null`. Repositories always take `tenantID` and add `WHERE tenant_id = $1`. Postgres RLS policies are enabled as defense in depth (`SET LOCAL app.tenant_id`). Platform admins use a separate role.
**Rejected**: schema-per-tenant (ops cost), DB-per-tenant (cost at thousands of small sellers).
