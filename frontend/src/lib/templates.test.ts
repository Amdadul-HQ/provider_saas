import { describe, expect, it } from "vitest"
import { renderTemplate, unfilledPlaceholders } from "./templates"

describe("renderTemplate (BUG-17)", () => {
  it("fills known variables case-insensitively", () => {
    const r = renderTemplate("ধন্যবাদ {{customer}}! {{ Product }} এর মেয়াদ {{expiry}}", { customer: "Fahim", product: "Netflix", expiry: "29 Oct" })
    expect(r.text).toBe("ধন্যবাদ Fahim! Netflix এর মেয়াদ 29 Oct")
    expect(r.missing).toEqual([])
  })

  it("supports aliases used by older templates", () => {
    expect(renderTemplate("{{name}} · {{expires_at}} · {{due}}", { customer: "A", expiry: "B", amount: "৳ 5" }).text).toBe("A · B · ৳ 5")
  })

  it("leaves unknown or empty variables in place and reports them", () => {
    const r = renderTemplate("{{customer}} {{amount}} {{link}}", { customer: "Rahim", amount: "" })
    expect(r.text).toBe("Rahim {{amount}} {{link}}")
    expect(r.missing).toEqual(["amount", "link"])
    expect(unfilledPlaceholders(r.text)).toEqual(["amount", "link"])
  })
})
