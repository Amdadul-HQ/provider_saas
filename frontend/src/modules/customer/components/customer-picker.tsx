"use client"

import { useState } from "react"
import { CheckIcon, ChevronsUpDownIcon, PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { useDebounced } from "@/hooks/use-debounced"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useCustomers } from "../hooks/use-customers"
import { CustomerFormDialog } from "./customer-form-dialog"

export type PickedCustomer = { id: string; name: string; phone?: string | null }

/** Searchable customer combobox with inline "new customer". */
export function CustomerPicker({
  value,
  onChange,
  id,
  invalid,
}: {
  value: PickedCustomer | null
  onChange: (customer: PickedCustomer | null) => void
  id?: string
  invalid?: boolean
}) {
  const t = useT()
  const [open, setOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [search, setSearch] = useState("")
  const debounced = useDebounced(search, 300)
  const [{ data, fetching }] = useCustomers({ search: debounced || undefined }, { first: 20 }, !open)
  const options = data?.customers.edges.map((e) => e.node) ?? []

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button id={id} variant="outline" role="combobox" aria-expanded={open} aria-invalid={invalid} className="w-full justify-between font-normal">
            <span className={cn("truncate", !value && "text-muted-foreground")}>
              {value ? `${value.name}${value.phone ? ` · ${value.phone}` : ""}` : t.customers.pick}
            </span>
            <ChevronsUpDownIcon className="opacity-50" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-(--radix-popover-trigger-width) min-w-72 p-0" align="start">
          <Command shouldFilter={false}>
            <CommandInput placeholder={t.customers.searchPlaceholder} value={search} onValueChange={setSearch} />
            <CommandList>
              <CommandEmpty>{fetching ? t.common.loading : t.customers.pickEmpty}</CommandEmpty>
              <CommandGroup>
                {options.map((c) => (
                  <CommandItem
                    key={c.id}
                    value={c.id}
                    onSelect={() => {
                      onChange({ id: c.id, name: c.name, phone: c.phone })
                      setOpen(false)
                    }}
                  >
                    <CheckIcon className={cn(value?.id === c.id ? "opacity-100" : "opacity-0")} />
                    <span className="flex-1 truncate">{c.name}</span>
                    <span className="text-xs text-muted-foreground">{c.phone}</span>
                  </CommandItem>
                ))}
              </CommandGroup>
              <CommandGroup>
                <CommandItem
                  value="__new__"
                  onSelect={() => {
                    setOpen(false)
                    setCreateOpen(true)
                  }}
                >
                  <PlusIcon /> {t.customers.new}
                </CommandItem>
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      <CustomerFormDialog open={createOpen} onOpenChange={setCreateOpen} onSaved={(c) => onChange({ id: c.id, name: c.name })} />
    </>
  )
}
