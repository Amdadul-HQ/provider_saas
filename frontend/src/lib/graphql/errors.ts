import type { CombinedError } from "@urql/core"
import type { Dict } from "@/i18n"

export type ErrorKind = "network" | "unauthenticated" | "forbidden" | "notFound" | "validation" | "generic"

type AnyError = CombinedError | Error | null | undefined

function isCombined(error: AnyError): error is CombinedError {
  return Boolean(error && "graphQLErrors" in error)
}

function httpStatus(error: AnyError): number | undefined {
  return isCombined(error) ? (error.response as { status?: number } | undefined)?.status : undefined
}

/** `extensions.code` of the first GraphQL error ("NETWORK" for transport failures), or null. */
export function errorCode(error: AnyError): string | null {
  if (!error) return null
  if (httpStatus(error) === 429) return "RATE_LIMITED"
  if (isCombined(error)) {
    const code = error.graphQLErrors.map((e) => e.extensions?.code).find((c) => typeof c === "string" && c !== "")
    if (typeof code === "string") return code
    if (error.networkError) return "NETWORK"
    if (httpStatus(error) === 401) return "UNAUTHENTICATED"
  }
  return null
}

/** `extensions.field` of the first GraphQL error that names one (a form field, or "account" for register), or null. */
export function errorField(error: AnyError): string | null {
  if (!isCombined(error)) return null
  const field = error.graphQLErrors.map((e) => e.extensions?.field).find((f) => typeof f === "string" && f !== "")
  return typeof field === "string" ? field : null
}

export function errorKind(error: AnyError): ErrorKind | null {
  if (!error) return null
  switch (errorCode(error)) {
    case "NETWORK":
      return "network"
    case "UNAUTHENTICATED":
      return "unauthenticated"
    case "FORBIDDEN":
      return "forbidden"
    case "NOT_FOUND":
      return "notFound"
    case "BAD_USER_INPUT":
    case "VALIDATION":
    case "GRAPHQL_VALIDATION_FAILED":
      return "validation"
    default:
      return "generic"
  }
}

/**
 * Where the error happened. Auth forms map UNAUTHENTICATED to "wrong credentials", never to
 * "session expired" (BUG-05): there is no session yet on those screens.
 */
export type ErrorContext = "default" | "login" | "invite"

/**
 * Friendly, translated message for any API error (BUG-07). Backend `message` text is never shown to
 * users — it is English, may leak internals, and is only exposed through `rawErrorMessage` in
 * development builds.
 */
export function describeError(t: Dict, error: AnyError, context: ErrorContext = "default"): string {
  if (!error) return t.errors.generic
  switch (errorCode(error)) {
    case "NETWORK":
      return t.errors.network
    case "UNAUTHENTICATED":
      if (context === "login") return t.errors.invalidCredentials
      if (context === "invite") return t.auth.inviteWrongPassword
      return t.errors.unauthenticated
    case "FORBIDDEN":
      return t.errors.forbidden
    case "NOT_FOUND":
      return context === "invite" ? t.auth.inviteInvalid : t.errors.notFound
    case "BAD_USER_INPUT":
    case "VALIDATION":
    case "GRAPHQL_VALIDATION_FAILED":
      return t.errors.validation
    case "CONFLICT":
      return t.errors.conflict
    case "RATE_LIMITED":
      return t.errors.rateLimited
    case "TIMEOUT":
      return t.errors.timeout
    default:
      return t.errors.generic
  }
}

/** Raw server/transport text for the developer "details" disclosure. Null outside development. */
export function rawErrorMessage(error: AnyError | string): string | null {
  if (process.env.NODE_ENV !== "development" || !error) return null
  if (typeof error === "string") return error
  if (isCombined(error)) {
    const parts = error.graphQLErrors.map((e) => `${String(e.extensions?.code ?? "")} ${e.message}`.trim())
    if (error.networkError) parts.push(error.networkError.message)
    return parts.join("\n") || error.message
  }
  return error.message
}
