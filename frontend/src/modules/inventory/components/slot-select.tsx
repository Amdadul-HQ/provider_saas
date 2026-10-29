"use client"

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { useT } from "@/i18n"
import { useFreeSlots } from "../hooks/use-inventory"

const AUTO = "__auto__"

/** Free-slot picker for a product. `null` = let the server auto-pick the next free slot. */
export function SlotSelect({
  productId,
  value,
  onChange,
  id,
  allowAuto = true,
}: {
  productId: string | null | undefined
  value: string | null
  onChange: (slotId: string | null) => void
  id?: string
  allowAuto?: boolean
}) {
  const t = useT()
  const [{ data, fetching }] = useFreeSlots(productId)
  if (!productId) return <p className="text-sm text-muted-foreground">{t.catalog.pickProduct}</p>
  if (!data && fetching) return <Skeleton className="h-10 w-full" />
  const slots = data?.freeSlots ?? []
  if (slots.length === 0) return <p className="rounded-lg bg-warning/15 p-3 text-sm text-warning-foreground dark:text-warning">{t.inventory.noFreeSlots}</p>
  return (
    <Select value={value ?? (allowAuto ? AUTO : "")} onValueChange={(v) => onChange(v === AUTO ? null : v)}>
      <SelectTrigger id={id} className="w-full">
        <SelectValue placeholder={t.inventory.pickSlot} />
      </SelectTrigger>
      <SelectContent>
        {allowAuto ? <SelectItem value={AUTO}>{t.inventory.autoSlot}</SelectItem> : null}
        {slots.map((s) => (
          <SelectItem key={s.id} value={s.id}>
            {t.inventory.slotNo(s.slotNo)}
            {s.label ? ` · ${s.label}` : ""}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
