/** REQ-006 price list (BDT, minor units). Yearly = 10 months. Display only until billing ships. */
export type PlanKey = "starter" | "business" | "pro"
export type Cycle = "monthly" | "yearly"

export const PLANS: { key: PlanKey; monthly: number; suggested?: boolean }[] = [
  { key: "starter", monthly: 49_900 },
  { key: "business", monthly: 129_900, suggested: true },
  { key: "pro", monthly: 299_900 },
]

export const YEARLY_MONTHS = 10

export function planPrice(monthly: number, cycle: Cycle) {
  return cycle === "monthly" ? monthly : monthly * YEARLY_MONTHS
}

export function yearlySaving(monthly: number) {
  return monthly * 12 - monthly * YEARLY_MONTHS
}
