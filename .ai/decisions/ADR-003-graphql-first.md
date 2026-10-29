# ADR-003 GraphQL-first
**Decision**: Frontend talks only GraphQL (`/graphql`). Each backend module owns a `schema.graphql`; gqlgen merges them. Webhooks (Meta, WhatsApp, Telegram, SMS forwarder) and file uploads are REST because third parties require it.
**Rule**: lists are paginated with cursor `Connection` pattern; filters are typed input objects; UI tables/forms derive from the schema via codegen.
