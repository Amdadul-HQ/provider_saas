import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { authPayload, headersOf, json, memoryStorage, mockFetch, opName } from "./test-utils"

// fresh module state (in-memory access token, zustand store, single-flight promise) per test
async function load() {
  vi.resetModules()
  const session = await import("./session")
  const tokens = await import("./tokens")
  return { ...session, ...tokens }
}

describe("cookie-mode session (SEC M6)", () => {
  let storage: Storage
  beforeEach(() => {
    storage = memoryStorage()
    vi.stubGlobal("window", { localStorage: storage })
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it("refreshes from the cookie with no argument, cookie headers and credentials", async () => {
    storage.setItem("psaas.session", "1")
    const calls = mockFetch(() => json({ data: { refreshToken: authPayload() } }))
    const s = await load()
    await s.bootstrapSession()

    expect(calls).toHaveLength(1)
    const [call] = calls
    expect(opName(call!)).toBe("RefreshToken")
    expect(call!.body.variables).toEqual({})
    expect(call!.init.credentials).toBe("include")
    expect(headersOf(call!)).toMatchObject({ "x-auth-mode": "cookie", "x-requested-with": "XMLHttpRequest" })
    expect(s.useSessionStore.getState().status).toBe("authenticated")
    expect(s.getAccessToken()).toMatch(/\./)
    // nothing secret persisted: only the hint
    expect(storage.getItem("psaas.rt")).toBeNull()
    expect(storage.getItem("psaas.session")).toBe("1")
  })

  it("does not call the API for a visitor that never logged in", async () => {
    const calls = mockFetch(() => json({}))
    const s = await load()
    await s.bootstrapSession()
    expect(calls).toHaveLength(0)
    expect(s.useSessionStore.getState().status).toBe("anonymous")
  })

  it("migrates a Sprint 1 localStorage refresh token once, then deletes it", async () => {
    storage.setItem("psaas.rt", "legacy-token")
    const calls = mockFetch(() => json({ data: { refreshToken: authPayload() } }))
    const s = await load()
    await s.bootstrapSession()

    expect(calls).toHaveLength(1)
    expect(calls[0]!.body.variables).toEqual({ refreshToken: "legacy-token" })
    expect(headersOf(calls[0]!)["x-auth-mode"]).toBe("cookie")
    expect(storage.getItem("psaas.rt")).toBeNull()
    expect(s.useSessionStore.getState().status).toBe("authenticated")

    // the next refresh uses the cookie only
    await s.refreshSession()
    expect(calls[1]!.body.variables).toEqual({})
  })

  it("falls back to the cookie when the legacy token is rejected", async () => {
    storage.setItem("psaas.rt", "revoked")
    const calls = mockFetch((call) =>
      call.body.variables?.refreshToken
        ? json({ data: null, errors: [{ message: "invalid", extensions: { code: "UNAUTHENTICATED" } }] })
        : json({ data: { refreshToken: authPayload() } }),
    )
    const s = await load()
    expect(await s.refreshSession()).toBe("ok")
    expect(calls.map((c) => c.body.variables)).toEqual([{ refreshToken: "revoked" }, {}])
    expect(storage.getItem("psaas.rt")).toBeNull()
  })

  it("keeps the legacy token when the API is unreachable (retry later)", async () => {
    storage.setItem("psaas.rt", "legacy-token")
    mockFetch(() => {
      throw new TypeError("fetch failed")
    })
    const s = await load()
    await s.bootstrapSession()
    expect(s.useSessionStore.getState().status).toBe("unreachable")
    expect(storage.getItem("psaas.rt")).toBe("legacy-token")
  })

  it("signs out (clears hint and memory token) when the cookie is invalid", async () => {
    storage.setItem("psaas.session", "1")
    mockFetch(() => json({ data: null, errors: [{ message: "x", extensions: { code: "UNAUTHENTICATED" } }] }))
    const s = await load()
    expect(await s.refreshSession()).toBe("invalid")
    expect(s.useSessionStore.getState().status).toBe("anonymous")
    expect(storage.getItem("psaas.session")).toBeNull()
    expect(s.getAccessToken()).toBeNull()
  })

  it("is single-flight: concurrent callers share one refresh", async () => {
    storage.setItem("psaas.session", "1")
    const calls = mockFetch(() => json({ data: { refreshToken: authPayload() } }))
    const s = await load()
    const [a, b] = await Promise.all([s.refreshSession(), s.refreshSession()])
    expect([a, b]).toEqual(["ok", "ok"])
    expect(calls).toHaveLength(1)
  })
})
