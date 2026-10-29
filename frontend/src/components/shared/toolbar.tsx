"use client"

import { useEffect, useState } from "react"
import { SearchIcon, XIcon } from "lucide-react"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { useDebounced } from "@/hooks/use-debounced"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

/** Debounced search box for `*Filter.search`. */
export function SearchInput({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  className?: string
}) {
  const t = useT()
  const [text, setText] = useState(value)
  const debounced = useDebounced(text, 350)
  useEffect(() => {
    if (debounced !== value) onChange(debounced)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fire only on debounced input
  }, [debounced])
  return (
    <div className={cn("relative col-span-2 w-full md:max-w-xs", className)}>
      <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
      <Input
        type="search"
        aria-label={placeholder ?? t.common.search}
        placeholder={placeholder ?? t.common.search}
        className="pr-9 pl-9"
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      {text ? (
        <button
          type="button"
          aria-label={t.common.clear}
          className="absolute top-1/2 right-2 flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:bg-muted"
          onClick={() => setText("")}
        >
          <XIcon className="size-4" />
        </button>
      ) : null}
    </div>
  )
}

const ALL = "__all__"

/** Select bound to an optional enum filter; "all" maps to `undefined`. */
export function FilterSelect<V extends string>({
  value,
  onChange,
  options,
  label,
  className,
}: {
  value: V | undefined | null
  onChange: (value: V | undefined) => void
  options: { value: V; label: string }[]
  label: string
  className?: string
}) {
  const t = useT()
  return (
    <Select value={value ?? ALL} onValueChange={(v) => onChange(v === ALL ? undefined : (v as V))}>
      <SelectTrigger aria-label={label} className={cn("w-full md:w-44", className)}>
        <SelectValue placeholder={label} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>
          {label}: {t.common.all}
        </SelectItem>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export function FilterToggle({
  checked,
  onChange,
  label,
  id,
}: {
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
  id: string
}) {
  return (
    <div className="flex h-10 items-center gap-2 rounded-lg border px-3 md:h-9">
      <Switch id={id} checked={checked} onCheckedChange={onChange} />
      <Label htmlFor={id} className="text-sm font-normal whitespace-nowrap">
        {label}
      </Label>
    </div>
  )
}
