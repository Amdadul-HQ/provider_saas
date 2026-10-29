"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import type { ColumnDef } from "@tanstack/react-table"
import { PlusIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { ConfirmDialog, DataTable, EmptyState, NumberField, ResponsiveDialog, SelectField, StatusBadge, TextField, DateText } from "@/components/shared"
import type { Role, TenantMemberFieldsFragment } from "@/gql/graphql"
import { ROLES } from "@/lib/enums"
import { RoleGate, useSession } from "@/lib/auth"
import { normalizePhone, v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useInviteMember, useRemoveMember, useTenantSettings, useUpdateMemberRole } from "../hooks/use-tenant"

function InviteMemberDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [invite, saving] = useInviteMember()
  const schema = useMemo(
    () =>
      z.object({
        name: v.requiredString(t),
        email: v.optionalEmail(t),
        phone: v.optionalPhone(t),
        role: z.enum(ROLES as [Role, ...Role[]]),
        commissionPct: v.optionalInt(t),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { name: "", email: "", phone: "", role: "STAFF", commissionPct: 0 } })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.settings.invite}
      footer={
        <Button form="invite-member" type="submit" disabled={saving}>
          {t.settings.invite}
        </Button>
      }
    >
      <form
        id="invite-member"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const ok = await invite({
            input: {
              name: values.name,
              email: values.email || null,
              phone: values.phone ? normalizePhone(values.phone) : null,
              role: values.role,
              commissionBps: Math.round((values.commissionPct ?? 0) * 100),
            },
          })
          if (ok) {
            form.reset()
            onOpenChange(false)
          }
        })}
      >
        <FieldGroup>
          <TextField control={form.control} name="name" label={t.common.name} required />
          <TextField control={form.control} name="phone" type="tel" label={t.common.phone} />
          <TextField control={form.control} name="email" type="email" label={t.common.email} />
          <SelectField
            control={form.control}
            name="role"
            label={t.settings.role}
            options={ROLES.filter((r) => r !== "OWNER").map((r) => ({ value: r, label: enumLabel(t, "Role", r) }))}
          />
          <NumberField control={form.control} name="commissionPct" label={t.settings.commission} min={0} max={100} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

export function MembersPanel() {
  const t = useT()
  const { user, hasRole } = useSession()
  const [{ data, fetching, error }, refetch] = useTenantSettings()
  const [updateRole] = useUpdateMemberRole()
  const [removeMember, removing] = useRemoveMember()
  const [inviteOpen, setInviteOpen] = useState(false)
  const [removing_, setRemoving] = useState<TenantMemberFieldsFragment | null>(null)
  const isOwner = hasRole("OWNER")

  const columns = useMemo<ColumnDef<TenantMemberFieldsFragment>[]>(
    () => [
      {
        id: "name",
        header: t.common.name,
        cell: ({ row }) => (
          <div>
            <p className="font-medium">{row.original.user.name}</p>
            <p className="text-xs text-muted-foreground">{row.original.user.email ?? row.original.user.phone}</p>
          </div>
        ),
      },
      {
        id: "role",
        header: t.settings.role,
        cell: ({ row }) =>
          isOwner && row.original.user.id !== user?.id ? (
            <Select value={row.original.role} onValueChange={(role) => void updateRole({ memberId: row.original.id, role: role as Role })}>
              <SelectTrigger size="sm" className="w-36" aria-label={t.settings.role}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLES.map((r) => (
                  <SelectItem key={r} value={r}>
                    {enumLabel(t, "Role", r)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            enumLabel(t, "Role", row.original.role)
          ),
      },
      { id: "commission", header: t.settings.commission, cell: ({ row }) => `${row.original.commissionBps / 100}%` },
      { id: "status", header: t.common.status, cell: ({ row }) => row.original.status },
      { id: "since", header: t.common.createdAt, cell: ({ row }) => <DateText value={row.original.createdAt} /> },
      {
        id: "actions",
        header: "",
        cell: ({ row }) =>
          isOwner && row.original.user.id !== user?.id ? (
            <Button variant="ghost" size="icon-sm" aria-label={t.settings.removeMember} onClick={() => setRemoving(row.original)}>
              <Trash2Icon />
            </Button>
          ) : null,
      },
    ],
    [t, isOwner, user?.id, updateRole],
  )

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="font-semibold">{t.settings.members}</h2>
        <RoleGate roles={["OWNER", "ADMIN"]}>
          <Button onClick={() => setInviteOpen(true)}>
            <PlusIcon /> {t.settings.invite}
          </Button>
        </RoleGate>
      </div>
      <DataTable
        columns={columns}
        data={data?.tenant.members}
        fetching={fetching}
        error={error}
        onRetry={() => refetch({ requestPolicy: "network-only" })}
        empty={<EmptyState title={t.settings.members} />}
        mobileCard={(m) => (
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate font-medium">{m.user.name}</p>
              <p className="truncate text-xs text-muted-foreground">{m.user.email ?? m.user.phone}</p>
            </div>
            <StatusBadge enum="Role" value={m.role} />
          </div>
        )}
      />
      <InviteMemberDialog open={inviteOpen} onOpenChange={setInviteOpen} />
      <ConfirmDialog
        open={removing_ !== null}
        onOpenChange={(o) => !o && setRemoving(null)}
        title={t.settings.removeMember}
        description={t.settings.removeConfirm}
        destructive
        loading={removing}
        onConfirm={async () => {
          if (removing_ && (await removeMember({ memberId: removing_.id }))) setRemoving(null)
        }}
      />
    </div>
  )
}
