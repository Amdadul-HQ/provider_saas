"use client"

import { useMemo } from "react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { ExternalLinkIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup, FieldLegend, FieldSet } from "@/components/ui/field"
import { CopyButton, ErrorState, ImagePicker, ListSkeleton, SectionCard, SelectField, SwitchField, TextareaField, TextField } from "@/components/shared"
import type { StorefrontFieldsFragment } from "@/gql/graphql"
import { MOBILE_WALLETS, PAYMENT_METHODS } from "@/lib/enums"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useStorefrontSettings, useUpsertStorefront } from "../hooks/use-storefront"
import { parsePaymentNumbers } from "../lib"

function EditorForm({ storefront }: { storefront: StorefrontFieldsFragment | null }) {
  const t = useT()
  const [save, saving] = useUpsertStorefront()
  const schema = useMemo(
    () =>
      z.object({
        slug: z.string().trim().regex(/^[a-z0-9-]{3,40}$/, t.errors.required),
        displayName: v.requiredString(t),
        tagline: v.optionalString(),
        logoUrl: v.optionalString(),
        coverUrl: v.optionalString(),
        primaryColor: z.string().regex(/^#[0-9a-fA-F]{6}$/).or(z.literal("")),
        whatsapp: v.optionalPhone(t),
        messengerUrl: v.optionalString(),
        telegram: v.optionalString(),
        paymentInstructions: v.optionalString(),
        paymentNumbers: z.array(z.object({ method: z.string(), number: v.phone(t), type: z.string() })),
        isPublished: z.boolean(),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const theme = (storefront?.theme ?? {}) as Record<string, unknown>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      slug: storefront?.slug ?? "",
      displayName: storefront?.displayName ?? "",
      tagline: storefront?.tagline ?? "",
      logoUrl: storefront?.logoUrl ?? "",
      coverUrl: storefront?.coverUrl ?? "",
      primaryColor: typeof theme.primaryColor === "string" ? theme.primaryColor : "",
      whatsapp: storefront?.whatsapp ?? "",
      messengerUrl: storefront?.messengerUrl ?? "",
      telegram: storefront?.telegram ?? "",
      paymentInstructions: storefront?.paymentInstructions ?? "",
      paymentNumbers: parsePaymentNumbers(storefront?.paymentNumbers).map((p) => ({ method: p.method, number: p.number, type: p.type ?? "personal" })),
      isPublished: storefront?.isPublished ?? false,
    },
  })
  const numbers = useFieldArray({ control: form.control, name: "paymentNumbers" })

  return (
    <form
      noValidate
      onSubmit={form.handleSubmit((values) =>
        save({
          input: {
            slug: values.slug,
            displayName: values.displayName,
            tagline: values.tagline || null,
            logoUrl: values.logoUrl || null,
            coverUrl: values.coverUrl || null,
            theme: { ...theme, primaryColor: values.primaryColor || undefined },
            whatsapp: values.whatsapp || null,
            messengerUrl: values.messengerUrl || null,
            telegram: values.telegram || null,
            paymentInstructions: values.paymentInstructions || null,
            paymentNumbers: values.paymentNumbers,
            isPublished: values.isPublished,
          },
        }),
      )}
    >
      <FieldGroup>
        <SwitchField control={form.control} name="isPublished" label={t.storefront.isPublished} />
        <div className="grid gap-4 sm:grid-cols-2">
          <TextField control={form.control} name="displayName" label={t.storefront.displayName} required />
          <TextField control={form.control} name="slug" label={t.storefront.slug} placeholder="my-shop" required />
          <TextField control={form.control} name="tagline" label={t.storefront.tagline} className="sm:col-span-2" />
          <TextField control={form.control} name="primaryColor" label={t.storefront.brandColor} placeholder="#1f6f6f" />
        </div>
        <div className="grid gap-4 sm:grid-cols-[auto_minmax(0,1fr)]">
          <Controller
            control={form.control}
            name="logoUrl"
            render={({ field }) => <ImagePicker label={t.storefront.logoUrl} value={field.value} onChange={(url) => field.onChange(url ?? "")} />}
          />
          <Controller
            control={form.control}
            name="coverUrl"
            render={({ field }) => <ImagePicker label={t.storefront.coverUrl} aspect="wide" value={field.value} onChange={(url) => field.onChange(url ?? "")} />}
          />
        </div>
        <FieldSet>
          <FieldLegend variant="label">{t.storefront.contactUs}</FieldLegend>
          <div className="grid gap-4 sm:grid-cols-3">
            <TextField control={form.control} name="whatsapp" type="tel" label={t.storefront.whatsapp} placeholder="01XXXXXXXXX" />
            <TextField control={form.control} name="messengerUrl" type="url" label={t.storefront.messengerUrl} placeholder="https://m.me/yourpage" />
            <TextField control={form.control} name="telegram" label={t.storefront.telegram} placeholder="@yourshop" />
          </div>
        </FieldSet>
        <FieldSet>
          <FieldLegend variant="label">{t.storefront.paymentNumbers}</FieldLegend>
          <div className="space-y-2">
            {numbers.fields.map((f, i) => (
              <div key={f.id} className="grid grid-cols-2 items-start gap-2 rounded-lg border p-3 sm:grid-cols-[9rem_1fr_9rem_auto]">
                <SelectField
                  control={form.control}
                  name={`paymentNumbers.${i}.method`}
                  label={t.billing.method}
                  options={PAYMENT_METHODS.filter((m) => MOBILE_WALLETS.includes(m) || m === "BANK").map((m) => ({ value: m, label: enumLabel(t, "PaymentMethod", m) }))}
                />
                <TextField control={form.control} name={`paymentNumbers.${i}.number`} type="tel" label={t.common.phone} />
                <SelectField
                  control={form.control}
                  name={`paymentNumbers.${i}.type`}
                  label={t.storefront.accountType}
                  options={[
                    { value: "personal", label: t.storefront.accountTypes.personal },
                    { value: "merchant", label: t.storefront.accountTypes.merchant },
                    { value: "agent", label: t.storefront.accountTypes.agent },
                  ]}
                />
                <Button type="button" variant="ghost" size="icon" className="self-end" aria-label={t.common.delete} onClick={() => numbers.remove(i)}>
                  <Trash2Icon />
                </Button>
              </div>
            ))}
            <Button type="button" variant="outline" onClick={() => numbers.append({ method: "BKASH", number: "", type: "personal" })}>
              <PlusIcon /> {t.storefront.addPaymentNumber}
            </Button>
          </div>
        </FieldSet>
        <TextareaField control={form.control} name="paymentInstructions" label={t.storefront.paymentInstructions} rows={4} />
        <RoleGate roles={["OWNER", "ADMIN"]}>
          <Button type="submit" disabled={saving} className="w-fit">
            {saving ? t.common.saving : t.common.save}
          </Button>
        </RoleGate>
      </FieldGroup>
    </form>
  )
}

export function StorefrontEditor() {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useStorefrontSettings()
  if (data === undefined) return error ? <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} /> : fetching ? <ListSkeleton rows={5} /> : null
  const sf = data.storefront
  return (
    <div className="space-y-4">
      {sf ? (
        <SectionCard>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">{t.storefront.publicUrl}</p>
              <p className="truncate font-mono text-sm">{sf.publicUrl}</p>
            </div>
            <div className="flex gap-2">
              <CopyButton value={sf.publicUrl} />
              <Button variant="outline" size="sm" asChild>
                <a href={`/s/${sf.slug}`} target="_blank" rel="noreferrer">
                  <ExternalLinkIcon /> {t.common.view}
                </a>
              </Button>
            </div>
          </div>
        </SectionCard>
      ) : null}
      <SectionCard title={t.storefront.editorTitle}>
        <EditorForm storefront={sf ?? null} />
      </SectionCard>
    </div>
  )
}
