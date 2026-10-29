"use client"

import { useMemo } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Loader2Icon, SendIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLegend, FieldSet } from "@/components/ui/field"
import { PlatformIcon, SectionCard, TextareaField, TextField } from "@/components/shared"
import { dateTimeInputToISO } from "@/lib/dates"
import { v } from "@/lib/validation"
import { useSocialAccounts } from "@/modules/social"
import { useT } from "@/i18n"
import { useCreatePost, usePublishPostNow } from "../hooks/use-content"

/** Write once, post to many pages; optional schedule (Dhaka time). */
export function PostComposer({ onSaved }: { onSaved?: () => void }) {
  const t = useT()
  const [{ data }] = useSocialAccounts()
  const [create, creating] = useCreatePost()
  const [publish, publishing] = usePublishPostNow()
  const accounts = (data?.socialAccounts ?? []).filter((a) => a.status === "CONNECTED")
  const schema = useMemo(
    () =>
      z.object({
        body: v.requiredString(t),
        mediaUrl: v.optionalString(),
        scheduledAt: v.optionalString(),
        socialAccountIds: z.array(z.string()).min(1, t.content.selectAtLeastOne),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { body: "", mediaUrl: "", scheduledAt: "", socialAccountIds: [] } })

  async function save(values: Values, publishNow: boolean) {
    const res = await create({
      input: {
        body: values.body,
        media: values.mediaUrl ? [{ type: "image", url: values.mediaUrl }] : [],
        scheduledAt: publishNow ? null : dateTimeInputToISO(values.scheduledAt),
        socialAccountIds: values.socialAccountIds,
      },
    })
    if (!res) return
    if (publishNow) await publish({ id: res.createPost.id })
    form.reset()
    onSaved?.()
  }

  return (
    <SectionCard title={t.content.composer}>
      <form noValidate onSubmit={form.handleSubmit((v) => save(v, false))}>
        <FieldGroup>
          <TextareaField control={form.control} name="body" label={t.content.body} placeholder={t.content.bodyPlaceholder} rows={5} required />
          <TextField control={form.control} name="mediaUrl" type="url" label={t.content.mediaUrl} />
          <Controller
            control={form.control}
            name="socialAccountIds"
            render={({ field, fieldState }) => (
              <FieldSet data-invalid={fieldState.invalid || undefined}>
                <FieldLegend variant="label">{t.content.targets}</FieldLegend>
                {accounts.length === 0 ? (
                  <FieldDescription>{t.content.noTargets}</FieldDescription>
                ) : (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {accounts.map((a) => {
                      const checked = field.value.includes(a.id)
                      return (
                        <Field key={a.id} orientation="horizontal" className="min-h-11 rounded-lg border px-3">
                          <Checkbox
                            id={`target-${a.id}`}
                            checked={checked}
                            onCheckedChange={(c) => field.onChange(c ? [...field.value, a.id] : field.value.filter((x) => x !== a.id))}
                          />
                          <label htmlFor={`target-${a.id}`} className="flex flex-1 items-center gap-2 text-sm">
                            <PlatformIcon platform={a.platform} className="size-5" />
                            <span className="truncate">{a.name}</span>
                          </label>
                        </Field>
                      )
                    })}
                  </div>
                )}
                <FieldError errors={[fieldState.error]} />
              </FieldSet>
            )}
          />
          <TextField control={form.control} name="scheduledAt" type="datetime-local" label={t.content.schedule} description={t.content.scheduleHint} />
          <div className="flex flex-wrap gap-2">
            <Button type="submit" variant="outline" disabled={creating || publishing}>
              {t.content.saveDraft}
            </Button>
            <Button type="button" disabled={creating || publishing} onClick={form.handleSubmit((v) => save(v, true))}>
              {publishing ? <Loader2Icon className="animate-spin" /> : <SendIcon />} {t.content.publishNow}
            </Button>
          </div>
        </FieldGroup>
      </form>
    </SectionCard>
  )
}
