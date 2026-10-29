"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import type { ColumnDef } from "@tanstack/react-table"
import { MessageSquareQuoteIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { ConfirmDialog, DataTable, EmptyState, FilterSelect, ResponsiveDialog, SelectField, SwitchField, TextareaField, TextField, ToneBadge } from "@/components/shared"
import type { MessageTemplateFieldsFragment, TemplateCategory } from "@/gql/graphql"
import { TEMPLATE_CATEGORIES } from "@/lib/enums"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useDeleteTemplate, useMessageTemplates, useUpsertTemplate } from "../hooks/use-inbox"

function TemplateDialog({ template, open, onOpenChange }: { template: MessageTemplateFieldsFragment | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [upsert, saving] = useUpsertTemplate()
  const schema = useMemo(
    () =>
      z.object({
        key: z.string().trim().regex(/^[a-z0-9_.-]+$/, t.errors.required),
        name: v.requiredString(t),
        body: v.requiredString(t),
        category: z.enum(TEMPLATE_CATEGORIES as [TemplateCategory, ...TemplateCategory[]]),
        isActive: z.boolean(),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      key: template?.key ?? "",
      name: template?.name ?? "",
      body: template?.body ?? "",
      category: template?.category ?? "QUICK_REPLY",
      isActive: template?.isActive ?? true,
    },
  })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={template ? template.name : t.settings.newTemplate}
      footer={
        <Button type="submit" form="template-form" disabled={saving}>
          {t.common.save}
        </Button>
      }
    >
      <form
        id="template-form"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          if (await upsert({ id: template?.id ?? null, input: values })) onOpenChange(false)
        })}
      >
        <FieldGroup>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField control={form.control} name="name" label={t.settings.templateName} required />
            <TextField control={form.control} name="key" label={t.settings.templateKey} placeholder="renew_reminder" required />
          </div>
          <SelectField
            control={form.control}
            name="category"
            label={t.settings.category}
            options={TEMPLATE_CATEGORIES.map((c) => ({ value: c, label: enumLabel(t, "TemplateCategory", c) }))}
          />
          <TextareaField control={form.control} name="body" label={t.settings.templateBody} description={t.settings.templateBodyHint} rows={5} />
          <SwitchField control={form.control} name="isActive" label={t.common.active} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

export function TemplateManager() {
  const t = useT()
  const [category, setCategory] = useState<TemplateCategory | undefined>()
  const [{ data, fetching, error }, refetch] = useMessageTemplates(category)
  const [editing, setEditing] = useState<MessageTemplateFieldsFragment | null>(null)
  const [open, setOpen] = useState(false)
  const [deleting, setDeleting] = useState<MessageTemplateFieldsFragment | null>(null)
  const [del, delBusy] = useDeleteTemplate()
  const openEditor = (tpl: MessageTemplateFieldsFragment | null) => {
    setEditing(tpl)
    setOpen(true)
  }
  const columns = useMemo<ColumnDef<MessageTemplateFieldsFragment>[]>(
    () => [
      {
        id: "name",
        header: t.settings.templateName,
        cell: ({ row }) => (
          <div>
            <p className="font-medium">{row.original.name}</p>
            <p className="font-mono text-xs text-muted-foreground">{row.original.key}</p>
          </div>
        ),
      },
      { id: "category", header: t.settings.category, cell: ({ row }) => enumLabel(t, "TemplateCategory", row.original.category) },
      { id: "body", header: t.settings.templateBody, cell: ({ row }) => <p className="line-clamp-2 max-w-md text-sm whitespace-normal text-muted-foreground">{row.original.body}</p> },
      { id: "active", header: "", cell: ({ row }) => (!row.original.isActive ? <ToneBadge>{t.common.inactive}</ToneBadge> : null) },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => (
          <RoleGate roles={["OWNER", "ADMIN"]}>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t.common.delete}
              onClick={(e) => {
                e.stopPropagation()
                setDeleting(row.original)
              }}
            >
              <Trash2Icon />
            </Button>
          </RoleGate>
        ),
      },
    ],
    [t],
  )
  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <FilterSelect<TemplateCategory>
          label={t.settings.category}
          value={category}
          onChange={setCategory}
          options={TEMPLATE_CATEGORIES.map((c) => ({ value: c, label: enumLabel(t, "TemplateCategory", c) }))}
        />
        <RoleGate roles={["OWNER", "ADMIN"]}>
          <Button onClick={() => openEditor(null)}>
            <PlusIcon /> {t.settings.newTemplate}
          </Button>
        </RoleGate>
      </div>
      <DataTable
        columns={columns}
        data={data?.messageTemplates}
        fetching={fetching}
        error={error}
        onRetry={() => refetch({ requestPolicy: "network-only" })}
        onRowClick={openEditor}
        empty={<EmptyState icon={MessageSquareQuoteIcon} title={t.settings.templatesEmpty} />}
        mobileCard={(tpl) => (
          <div className="space-y-1">
            <div className="flex items-center justify-between gap-2">
              <p className="font-medium">{tpl.name}</p>
              <span className="text-xs text-muted-foreground">{enumLabel(t, "TemplateCategory", tpl.category)}</span>
            </div>
            <p className="line-clamp-2 text-xs text-muted-foreground">{tpl.body}</p>
          </div>
        )}
      />
      <TemplateDialog template={editing} open={open} onOpenChange={setOpen} />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={t.common.delete}
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
