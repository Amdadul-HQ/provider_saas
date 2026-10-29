import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { authPayload, fakeJwt, headersOf, json, memoryStorage, mockFetch, opName, TENANT_ID } from "@/lib/auth/test-utils"

async function load() {
  vi.resetModules()
  const { makeClient } = await import("./client")
  const session = await import("@/lib/auth/session")
  const gql = await import("@/gql/graphql")
  return { makeClient, session, gql }
}

const unauthenticated = () => json({ data: null, errors: [{ message: "unauthenticated", extensions: { code: "UNAUTHENTICATED" } }] })

describe("urql auth exchange in cookie mode", () => {
  let storage: Storage
  beforeEach(() => {
    storage = memoryStorage({ "psaas.session": "1" })
    vi.stubGlobal("window", { localStorage: storage })
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it("sends cookie-mode headers, credentials and the bearer token on every request", async () => {
    const { makeClient, session, gql } = await load()
    session.useSessionStore.getState().setAuthenticated(authPayload())
    const calls = mockFetch(() => json({ data: { inboxCounts: { __typename: "InboxCounts", open: 1, unassigned: 0, mine: 0, unread: 2 } } }))
    const res = await makeClient().query(gql.InboxCountsDocument, {}).toPromise()

    expect(res.data?.inboxCounts.unread).toBe(2)
    expect(calls).toHaveLength(1)
    const h = headersOf(calls[0]!)
    expect(h["x-auth-mode"]).toBe("cookie")
    expect(h["x-requested-with"]).toBe("XMLHttpRequest")
    expect(h.authorization).toMatch(/^Bearer /)
    expect(calls[0]!.init.credentials).toBe("include")
    expect(calls[0]!.init.method).toBe("POST")
  })

  it("on UNAUTHENTICATED refreshes from the cookie (no argument) and retries once", async () => {
    const { makeClient, session, gql } = await load()
    session.useSessionStore.getState().setAuthenticated(authPayload())
    const fresh = fakeJwt({ exp: Math.floor(Date.now() / 1000) + 900, tenant_id: TENANT_ID, v: 2 })
    let failedOnce = false
    const calls = mockFetch((call) => {
      if (opName(call) === "RefreshToken") return json({ data: { refreshToken: authPayload(fresh) } })
      if (!failedOnce) {
        failedOnce = true
        return unauthenticated()
      }
      return json({ data: { inboxCounts: { __typename: "InboxCounts", open: 0, unassigned: 0, mine: 0, unread: 0 } } })
    })
    const res = await makeClient().query(gql.InboxCountsDocument, {}).toPromise()

    expect(res.error).toBeUndefined()
    expect(calls.map(opName)).toEqual(["InboxCounts", "RefreshToken", "InboxCounts"])
    expect(calls[1]!.body.variables).toEqual({})
    expect(headersOf(calls[2]!).authorization).toBe(`Bearer ${fresh}`)
  })

  it("on HTTP 401 also refreshes and signs out when the cookie is gone", async () => {
    const { makeClient, session, gql } = await load()
    session.useSessionStore.getState().setAuthenticated(authPayload())
    const calls = mockFetch((call) => (opName(call) === "RefreshToken" ? unauthenticated() : json({ errors: [{ message: "no" }] }, 401)))
    await makeClient().query(gql.InboxCountsDocument, {}).toPromise()

    expect(calls.map(opName)).toContain("RefreshToken")
    expect(session.useSessionStore.getState().status).toBe("anonymous")
    expect(storage.getItem("psaas.session")).toBeNull()
  })

  it("never refreshes + retries a failed public auth mutation (wrong password)", async () => {
    const { makeClient, gql } = await load()
    const calls = mockFetch(() => unauthenticated())
    const res = await makeClient()
      .mutation(gql.LoginDocument, { input: { identifier: "x@y.z", password: "bad" } })
      .toPromise()
    expect(res.error?.graphQLErrors[0]?.extensions?.code).toBe("UNAUTHENTICATED")
    expect(calls.map(opName)).toEqual(["Login"])
    expect(headersOf(calls[0]!).authorization).toBeUndefined()
    expect(headersOf(calls[0]!)["x-auth-mode"]).toBe("cookie")
  })

  it("logout and logoutAll take no refresh-token argument", async () => {
    const { makeClient, session, gql } = await load()
    session.useSessionStore.getState().setAuthenticated(authPayload())
    const calls = mockFetch((call) => json({ data: opName(call) === "Logout" ? { logout: true } : { logoutAll: true } }))
    const client = makeClient()
    await client.mutation(gql.LogoutDocument, {}).toPromise()
    await client.mutation(gql.LogoutAllDocument, {}).toPromise()
    expect(calls.map((c) => c.body.variables ?? {})).toEqual([{}, {}])
    expect(calls[0]!.init.credentials).toBe("include")
    expect(headersOf(calls[1]!).authorization).toMatch(/^Bearer /)
  })
})
