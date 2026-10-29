export const GRAPHQL_URL = process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/graphql"
export const APP_URL = process.env.NEXT_PUBLIC_APP_URL ?? "http://localhost:3000"

/**
 * API base for the REST endpoints next to /graphql (`POST /uploads`, `GET /events`).
 * Defaults to the GraphQL URL without its `/graphql` path, so one env var configures both.
 */
export const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? GRAPHQL_URL.replace(/\/graphql\/?$/, "")).replace(/\/$/, "")

/** Absolute URL for an API path (`/uploads`, `/events`); absolute inputs are returned unchanged. */
export function apiUrl(path: string): string {
  if (/^https?:\/\//i.test(path)) return path
  return `${API_URL}${path.startsWith("/") ? "" : "/"}${path}`
}
