import type { CombinedError } from "@urql/core"

export type ErrorKind = "network" | "unauthenticated" | "forbidden" | "notFound" | "validation" | "generic"

export function errorKind(error: CombinedError | Error | null | undefined): ErrorKind | null {
  if (!error) return null
  if ("networkError" in error && error.networkError) return "network"
  if ("graphQLErrors" in error) {
    const codes = error.graphQLErrors.map((e) => String(e.extensions?.code ?? ""))
    if (codes.includes("UNAUTHENTICATED")) return "unauthenticated"
    if (codes.includes("FORBIDDEN")) return "forbidden"
    if (codes.includes("NOT_FOUND")) return "notFound"
    if (codes.some((c) => c === "BAD_USER_INPUT" || c === "VALIDATION" || c === "GRAPHQL_VALIDATION_FAILED")) return "validation"
    const res = (error as CombinedError).response as { status?: number } | undefined
    if (res?.status === 401) return "unauthenticated"
  }
  return "generic"
}

/** Server-provided message when it is meant for humans (validation), else null. */
export function serverMessage(error: CombinedError | Error | null | undefined): string | null {
  if (!error) return null
  if ("graphQLErrors" in error && error.graphQLErrors.length > 0) {
    return error.graphQLErrors[0]?.message ?? null
  }
  return null
}
