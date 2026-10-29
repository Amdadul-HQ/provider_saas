import { describe, expect, it } from "vitest"
import { safeNext } from "./safe-next"

const origin = "https://app.sellerdesk.test"

describe("safeNext (M7 open redirect)", () => {
  it("keeps same-origin relative paths", () => {
    expect(safeNext("/customers/42?tab=invoices#x", origin)).toBe("/customers/42?tab=invoices#x")
    expect(safeNext("/inbox", origin)).toBe("/inbox")
  })

  it.each([
    null,
    "",
    "dashboard",
    "//evil.com",
    "/\\evil.com",
    "/\\/evil.com",
    "\\\\evil.com",
    "https://evil.com",
    "javascript:alert(1)",
    "/\t/evil.com",
    "/\n/evil.com",
    "/%5Cevil.com/../x\\y",
  ])("rejects %j", (next) => {
    expect(safeNext(next, origin)).toBe("/dashboard")
  })
})
