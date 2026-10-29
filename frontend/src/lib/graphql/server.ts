import { print } from "graphql"
import type { TypedDocumentNode } from "@graphql-typed-document-node/core"
import { GRAPHQL_URL } from "@/lib/config"

/** Server-to-server API origin; defaults to the public one (set GRAPHQL_INTERNAL_URL behind a proxy). */
const SERVER_GRAPHQL_URL = process.env.GRAPHQL_INTERNAL_URL ?? GRAPHQL_URL

/**
 * Minimal anonymous GraphQL fetch for Server Components / `generateMetadata` (public operations only).
 * Never throws: metadata must not take a page down, so any failure returns null and callers fall back.
 */
export async function serverQuery<Data, Vars extends Record<string, unknown>>(
  document: TypedDocumentNode<Data, Vars>,
  variables: Vars,
  opts: { revalidate?: number | false; timeoutMs?: number } = {},
): Promise<Data | null> {
  const { revalidate = 60, timeoutMs = 2500 } = opts
  try {
    const res = await fetch(SERVER_GRAPHQL_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ query: print(document), variables }),
      signal: AbortSignal.timeout(timeoutMs),
      // revalidate:false → never store the response in Next's data cache (token-bearing requests)
      ...(revalidate === false ? { cache: "no-store" as const } : { next: { revalidate } }),
    })
    if (!res.ok) return null
    const json = (await res.json()) as { data?: Data | null }
    return json.data ?? null
  } catch {
    return null
  }
}
