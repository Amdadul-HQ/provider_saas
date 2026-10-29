"use client"

import { formatMoney } from "@/lib/money"
import { useDigits } from "@/i18n"
import { cn } from "@/lib/utils"

type Tone = "default" | "money" | "due" | "profit" | "muted"

const toneClass: Record<Tone, string> = {
  default: "",
  money: "text-money",
  due: "text-destructive",
  profit: "text-success",
  muted: "text-muted-foreground",
}

/** Renders integer minor units as BDT: 35000 → "৳ 350". */
export function MoneyText({
  amount,
  currency = "BDT",
  tone = "default",
  compact,
  className,
}: {
  amount: number | null | undefined
  currency?: string
  tone?: Tone
  compact?: boolean
  className?: string
}) {
  const digits = useDigits()
  return (
    <span className={cn("tabular whitespace-nowrap font-medium", toneClass[tone], className)}>
      {formatMoney(amount, { currency, digits, compact })}
    </span>
  )
}
