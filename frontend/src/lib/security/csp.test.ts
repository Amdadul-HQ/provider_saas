import { describe, expect, it } from "vitest"
import { buildCsp } from "./csp"

describe("buildCsp (M6)", () => {
  it("production: nonce + strict-dynamic, no unsafe script sources", () => {
    const csp = buildCsp({ nonce: "abc", dev: false, apiUrl: "https://api.sellerdesk.test/graphql" })
    expect(csp).toContain("script-src 'self' 'nonce-abc' 'strict-dynamic'")
    expect(csp).not.toMatch(/script-src[^;]*unsafe/)
    expect(csp).toContain("connect-src 'self' https://api.sellerdesk.test;")
    expect(csp).toContain("frame-ancestors 'none'")
    expect(csp).toContain("default-src 'self'")
  })

  it("development allows eval/inline for React Refresh and HMR websockets", () => {
    const csp = buildCsp({ nonce: "abc", dev: true, apiUrl: "http://localhost:8080/graphql" })
    expect(csp).toContain("'unsafe-eval'")
    expect(csp).not.toContain("nonce-")
    expect(csp).toContain("connect-src 'self' http://localhost:8080 ws:")
  })
})
