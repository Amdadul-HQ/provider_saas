# ADR-005 Module layout
Backend `internal/modules/<m>/`: `domain.go` (types), `repository.go` (SQL via pgx), `service.go` (business rules), `resolver.go` (GraphQL glue), `<module>.graphql` (unique names so gqlgen emits `graph/<module>.resolvers.go`). Modules export a `Service` interface; other modules depend on the interface, wired in `internal/app/container.go`.
Frontend `src/modules/<m>/`: `components/`, `hooks/`, `graphql/*.graphql`, `index.ts`. Pages in `app/` only compose module components.
