"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import type { ColumnDef } from "@tanstack/react-table"
import { PencilIcon, PlusIcon, StarIcon, Trash2Icon, TruckIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { ConfirmDialog, DataTable, EmptyState, NumberField, ResponsiveDialog, TextareaField, TextField } from "@/components/shared"
import type { SupplierFieldsFragment } from "@/gql/graphql"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { useT } from "@/i18n"
import { useDeleteSupplier, useSaveSupplier, useSuppliers } from "../hooks/use-inventory"

function SupplierDialog({ supplier, open, onOpenChange }: { supplier: SupplierFieldsFragment | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const { create, update } = useSaveSupplier()
  const schema = useMemo(
    () => z.object({ name: v.requiredString(t), contact: v.optionalString(), notes: v.optionalString(), rating: z.number().int().min(1).max(5).nullable().optional() }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: { name: supplier?.name ?? "", contact: supplier?.contact ?? "", notes: supplier?.notes ?? "", rating: supplier?.rating ?? null },
  })
  const saving = create[1] || update[1]
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={supplier ? `${t.common.edit}: ${supplier.name}` : t.inventory.newSupplier}
      footer={
        <Button type="submit" form="supplier-form" disabled={saving}>
          {saving ? t.common.saving : t.common.save}
        </Button>
      }
    >
      <form
        id="supplier-form"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const input = { name: values.name, contact: values.contact || null, notes: values.notes || null, rating: values.rating ?? null }
          const ok = supplier ? await update[0]({ id: supplier.id, input }) : await create[0]({ input })
          if (ok) onOpenChange(false)
        })}
      >
        <FieldGroup>
          <TextField control={form.control} name="name" label={t.common.name} required />
          <TextField control={form.control} name="contact" label={t.inventory.supplierContact} placeholder="WhatsApp / Telegram" />
          <NumberField control={form.control} name="rating" label={t.inventory.rating} min={1} max={5} />
          <TextareaField control={form.control} name="notes" label={t.common.notes} rows={2} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

export function SupplierPanel() {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useSuppliers()
  const [editing, setEditing] = useState<SupplierFieldsFragment | null>(null)
  const [open, setOpen] = useState(false)
  const [deleting, setDeleting] = useState<SupplierFieldsFragment | null>(null)
  const [del, delBusy] = useDeleteSupplier()
  const openEditor = (s: SupplierFieldsFragment | null) => {
    setEditing(s)
    setOpen(true)
  }

  const columns = useMemo<ColumnDef<SupplierFieldsFragment>[]>(
    () => [
      { id: "name", header: t.common.name, cell: ({ row }) => <span className="font-medium">{row.original.name}</span> },
      { id: "contact", header: t.inventory.supplierContact, cell: ({ row }) => row.original.contact ?? "—" },
      {
        id: "rating",
        header: t.inventory.rating,
        cell: ({ row }) =>
          row.original.rating ? (
            <span className="flex items-center gap-1 text-warning-foreground dark:text-warning">
              <StarIcon className="size-3.5 fill-current" /> {row.original.rating}
            </span>
          ) : (
            "—"
          ),
      },
      { id: "accounts", header: t.inventory.tabs.accounts, cell: ({ row }) => t.inventory.accountCount(row.original.accountCount) },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => (
          <RoleGate roles={["OWNER", "ADMIN"]}>
            <div className="flex justify-end gap-1">
              <Button variant="ghost" size="icon-sm" aria-label={t.common.edit} onClick={() => openEditor(row.original)}>
                <PencilIcon />
              </Button>
              <Button variant="ghost" size="icon-sm" aria-label={t.common.delete} onClick={() => setDeleting(row.original)}>
                <Trash2Icon />
              </Button>
            </div>
          </RoleGate>
        ),
      },
    ],
    [t],
  )

  return (
    <div className="space-y-3">
      <RoleGate roles={["OWNER", "ADMIN"]}>
        <Button onClick={() => openEditor(null)}>
          <PlusIcon /> {t.inventory.newSupplier}
        </Button>
      </RoleGate>
      <DataTable
        columns={columns}
        data={data?.suppliers}
        fetching={fetching}
        error={error}
        onRetry={() => refetch({ requestPolicy: "network-only" })}
        empty={<EmptyState icon={TruckIcon} title={t.inventory.suppliersEmpty} />}
        onRowClick={(s) => openEditor(s)}
        mobileCard={(s) => (
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate font-medium">{s.name}</p>
              <p className="truncate text-xs text-muted-foreground">{s.contact ?? "—"}</p>
            </div>
            <span className="text-xs text-muted-foreground">{t.inventory.accountCount(s.accountCount)}</span>
          </div>
        )}
      />
      <SupplierDialog supplier={editing} open={open} onOpenChange={setOpen} />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={t.inventory.supplierDeleteConfirm}
        description={deleting?.name}
        destructive
        loading={delBusy}
        onConfirm={async () => {
          if (deleting && (await del({ id: deleting.id }))) setDeleting(null)
        }}
      />
    </div>
  )
}
