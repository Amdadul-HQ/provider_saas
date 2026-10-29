/** Test helpers for the auth/transport tests (vitest, node environment). Not imported by app code. */
import { vi } from "vitest"

export function memoryStorage(initial: Record<string, string> = {}): Storage {
  const map = new Map(Object.entries(initial))
  return {
    get length() {
      return map.size
    },
    clear: () => map.clear(),
    getItem: (k) => map.get(k) ?? null,
    key: (i) => Array.from(map.keys())[i] ?? null,
    removeItem: (k) => void map.delete(k),
    setItem: (k, v) => void map.set(k, String(v)),
  }
}

export function fakeJwt(claims: Record<string, unknown>): string {
  const b64 = (o: unknown) => btoa(JSON.stringify(o)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
  return `${b64({ alg: "HS256", typ: "JWT" })}.${b64(claims)}.sig`
}

export const TENANT_ID = "11111111-1111-1111-1111-111111111111"

export function sessionUser() {
  return {
    __typename: "User",
    id: "u1",
    name: "Rahim",
    email: "rahim@example.test",
    phone: null,
    avatarUrl: null,
    isPlatformAdmin: false,
    memberships: [
      {
        __typename: "Membership",
        id: "m1",
        role: "OWNER" as const,
        status: "active",
        commissionBps: 0,
        tenant: { __typename: "Tenant", id: TENANT_ID, name: "Shop", slug: "shop", currency: "BDT", timezone: "Asia/Dhaka", locale: "bn" },
      },
    ],
  }
}

export function authPayload(accessToken = fakeJwt({ exp: Math.floor(Date.now() / 1000) + 900, tenant_id: TENANT_ID })) {
  // cookie mode: refreshToken is null in the body
  return { __typename: "AuthPayload", accessToken, refreshToken: null, user: sessionUser() }
}

export function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } })
}

export type FetchCall = { url: string; init: RequestInit; body: { query?: string; operationName?: string; variables?: Record<string, unknown> } }

/** Installs a fetch mock; `handler` answers each call. Returns the recorded calls. */
export function mockFetch(handler: (call: FetchCall, index: number) => Response | Promise<Response>) {
  const calls: FetchCall[] = []
  const fn = vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const body = typeof init.body === "string" ? (JSON.parse(init.body) as FetchCall["body"]) : {}
    const call = { url: String(input), init, body }
    calls.push(call)
    return handler(call, calls.length - 1)
  })
  vi.stubGlobal("fetch", fn)
  return calls
}

/** Lower-cased header map of a recorded call. */
export function headersOf(call: FetchCall): Record<string, string> {
  const out: Record<string, string> = {}
  new Headers(call.init.headers).forEach((v, k) => {
    out[k] = v
  })
  return out
}

export function opName(call: FetchCall): string | undefined {
  return call.body.operationName ?? /(?:query|mutation)\s+(\w+)/.exec(call.body.query ?? "")?.[1]
}
