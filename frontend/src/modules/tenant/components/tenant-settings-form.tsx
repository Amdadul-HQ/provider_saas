"use client"

import { useMemo } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { DateText, DetailRow, ErrorState, ListSkeleton, SectionCard, SelectField, TextField } from "@/components/shared"
import type { TenantSettingsQuery } from "@/gql/graphql"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { useT, labelFor } from "@/i18n"
import { useTenantSettings, useUpdateTenant } from "../hooks/use-tenant"

type Tenant = TenantSettingsQuery["tenant"]

function Form({ tenant }: { tenant: Tenant }) {
  const t = useT()
  const [update, saving] = useUpdateTenant()
  const schema = useMemo(
    () => z.object({ name: v.requiredString(t), currency: v.requiredString(t), timezone: v.requiredString(t), locale: z.enum(["bn", "en"]) }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: tenant.name,
      currency: tenant.currency,
      timezone: tenant.timezone,
      locale: tenant.locale === "en" ? "en" : "bn",
    },
  })
  return (
    <form noValidate onSubmit={form.handleSubmit((values) => void update({ input: values }))}>
      <FieldGroup className="max-w-xl">
        <TextField control={form.control} name="name" label={t.settings.businessName} required />
        <div className="grid gap-4 sm:grid-cols-2">
          <SelectField control={form.control} name="currency" label={t.settings.currency} options={[{ value: "BDT", label: "BDT (৳)" }, { value: "USD", label: "USD ($)" }]} />
          <SelectField control={form.control} name="timezone" label={t.settings.timezone} options={[{ value: "Asia/Dhaka", label: "Asia/Dhaka (GMT+6)" }]} />
        </div>
        <SelectField control={form.control} name="locale" label={t.settings.locale} options={[{ value: "bn", label: "বাংলা" }, { value: "en", label: "English" }]} />
        <RoleGate roles={["OWNER", "ADMIN"]}>
          <Button type="submit" disabled={saving} className="w-fit">
            {saving ? t.common.saving : t.common.save}
          </Button>
        </RoleGate>
      </FieldGroup>
    </form>
  )
}

export function TenantSettingsForm() {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useTenantSettings()
  if (!data) return error ? <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} /> : fetching ? <ListSkeleton rows={4} /> : null
  return (
    <div className="grid gap-4 lg:grid-cols-[1fr_18rem]">
      <SectionCard title={t.settings.tabs.business}>
        <Form tenant={data.tenant} />
      </SectionCard>
      <SectionCard>
        <dl>
          <DetailRow label={t.settings.slug}>{data.tenant.slug}</DetailRow>
          <DetailRow label={t.settings.plan}>{labelFor(t, "TenantPlan", data.tenant.plan)}</DetailRow>
          <DetailRow label={t.common.status}>{labelFor(t, "TenantStatus", data.tenant.status)}</DetailRow>
          <DetailRow label={t.settings.trialEnds}>
            <DateText value={data.tenant.trialEndsAt} />
          </DetailRow>
        </dl>
      </SectionCard>
    </div>
  )
}
