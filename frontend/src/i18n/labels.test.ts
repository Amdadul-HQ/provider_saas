import { afterEach, describe, expect, it, vi } from "vitest"
import { getDictionary, getLocalizedDictionary, humanize, labelFor } from "./index"

const bn = getDictionary("bn")

describe("labelFor (BUG-06)", () => {
  afterEach(() => {
    vi.unstubAllEnvs()
    vi.restoreAllMocks()
  })

  it("translates string-typed API codes", () => {
    expect(labelFor(bn, "AtRiskReason", "EXPIRING_SOON")).toBe("শীঘ্রই মেয়াদ শেষ")
    expect(labelFor(bn, "AtRiskReason", "OVERDUE_INVOICE")).toBe("বিল মেয়াদোত্তীর্ণ")
    expect(labelFor(bn, "AtRiskReason", "DUE_BALANCE")).toBe("টাকা বাকি")
    expect(labelFor(bn, "TicketReason", "password_changed")).toBe("পাসওয়ার্ড বদলে গেছে")
    expect(labelFor(bn, "PaymentNumberType", "personal")).toBe("পার্সোনাল")
    expect(labelFor(bn, "TenantPlan", "free")).toBe("ফ্রি")
    expect(labelFor(bn, "MemberStatus", "invited")).toBe("আমন্ত্রিত")
  })

  it("maps lower-case DB values onto schema enums", () => {
    expect(labelFor(bn, "PaymentMethod", "bkash")).toBe("বিকাশ")
    expect(labelFor(bn, "SlotStatus", "free")).toBe("খালি")
    expect(labelFor(bn, "SlotStatus", "retired")).toBe("অবসরে")
    expect(labelFor(bn, "OrderSource", "storefront")).toBe("স্টোরফ্রন্ট")
    expect(labelFor(bn, "ReminderTrigger", "before_expiry")).toBe("মেয়াদ শেষের আগে")
  })

  it("falls back to a humanised string and warns once in development", () => {
    vi.stubEnv("NODE_ENV", "development")
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {})
    expect(labelFor(bn, "TicketReason", "screen_limit_hit")).toBe("Screen limit hit")
    labelFor(bn, "TicketReason", "screen_limit_hit")
    expect(warn).toHaveBeenCalledTimes(1)
  })

  it("passes free text through untouched and renders empty as a dash", () => {
    expect(labelFor(bn, "TicketReason", "প্রোফাইল লক")).toBe("প্রোফাইল লক")
    expect(labelFor(bn, "TicketReason", null)).toBe("—")
    expect(humanize("NOT_WORKING")).toBe("Not working")
  })
})

describe("digit-localised dictionary (BUG-08)", () => {
  it("rewrites static strings and template output to one numbering system", () => {
    const latn = getLocalizedDictionary("bn", "latn")
    const beng = getLocalizedDictionary("bn", "beng")
    expect(latn.dashboard.expiringIn7Days).toBe("7 দিনে মেয়াদ শেষ")
    expect(beng.common.daysLeft(2)).toBe("২ দিন বাকি")
    expect(latn.common.daysLeft(2)).toBe("2 দিন বাকি")
  })

  it("never rewrites string arguments (names, codes) or literal input formats", () => {
    const beng = getLocalizedDictionary("bn", "beng")
    expect(beng.inbox.via("NF-03 Page")).toBe("NF-03 Page এর মাধ্যমে")
    expect(beng.auth.identifierPlaceholder).toContain("01XXXXXXXXX")
  })
})

describe("localizeDigits", () => {
  it("keeps arrays as arrays", async () => {
    const { localizeDigits } = await import("./index")
    const out = localizeDigits({ list: [{ a: "1" }, { a: "2" }] }, "beng")
    expect(Array.isArray(out.list)).toBe(true)
    expect(out.list).toHaveLength(2)
    expect(out.list[1].a).toBe("২")
  })
})
