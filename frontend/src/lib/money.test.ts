import { describe, expect, it } from "vitest"
import { formatMoney, formatNumber, formatPercent, fromMinor, toMinor } from "./money"

describe("money", () => {
  it("converts between paisa and taka without floats leaking", () => {
    expect(toMinor("350.5")).toBe(35050)
    expect(toMinor("৳ 1,200")).toBe(120000)
    expect(toMinor("abc")).toBe(0)
    expect(fromMinor(35050)).toBe(350.5)
  })

  it("formats BDT with lakh grouping and only the fraction it needs", () => {
    expect(formatMoney(35000)).toBe("৳ 350")
    expect(formatMoney(35050)).toBe("৳ 350.50")
    expect(formatMoney(-1000)).toBe("-৳ 10")
    expect(formatMoney(1234567800)).toBe("৳ 1,23,45,678")
    expect(formatMoney(null)).toBe("৳ 0")
  })

  it("follows the digit setting (BUG-08)", () => {
    expect(formatMoney(1119000, { digits: "beng" })).toBe("৳ ১১,১৯০")
    expect(formatMoney(1119000, { digits: "latn" })).toBe("৳ 11,190")
    expect(formatNumber(4, "beng")).toBe("৪")
    expect(formatNumber(1234, "latn")).toBe("1,234")
    expect(formatPercent(0.456, "latn")).toBe("46%")
    expect(formatNumber(Number.NaN)).toBe("—")
  })
})
