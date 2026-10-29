import { describe, expect, it } from "vitest"
import { toDigits } from "./digits"
import { formatDate } from "./dates"

describe("digits", () => {
  it("rewrites both directions", () => {
    expect(toDigits("৭ দিনে 30", "latn")).toBe("7 দিনে 30")
    expect(toDigits("৭ দিনে 30", "beng")).toBe("৭ দিনে ৩০")
  })

  it("dates follow the setting, not the language", () => {
    const iso = "2026-10-29T06:00:00Z"
    expect(formatDate(iso, "bn", "latn")).toMatch(/29/)
    expect(formatDate(iso, "bn", "latn")).not.toMatch(/[০-৯]/)
    expect(formatDate(iso, "bn", "beng")).toMatch(/২৯/)
  })
})
