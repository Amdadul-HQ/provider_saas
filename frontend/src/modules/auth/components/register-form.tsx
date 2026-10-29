"use client"

import { useMemo } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Loader2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { TextField } from "@/components/shared"
import { describeError } from "@/lib/graphql/use-action"
import { normalizePhone, v } from "@/lib/validation"
import { useT } from "@/i18n"
import { useRegister } from "../hooks/use-auth"

export function RegisterForm({ onSuccess }: { onSuccess: () => void }) {
  const t = useT()
  const { register, fetching, error } = useRegister()
  const schema = useMemo(
    () =>
      z
        .object({
          name: v.requiredString(t),
          businessName: v.requiredString(t),
          phone: v.optionalPhone(t),
          email: v.optionalEmail(t),
          password: z.string().min(8, t.errors.minLength(8)),
        })
        .refine((d) => Boolean(d.phone?.trim()) || Boolean(d.email?.trim()), { message: t.auth.emailOrPhoneRequired, path: ["phone"] }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", businessName: "", phone: "", email: "", password: "" },
  })

  return (
    <form
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        const res = await register({
          name: values.name,
          businessName: values.businessName,
          password: values.password,
          phone: values.phone ? normalizePhone(values.phone) : null,
          email: values.email || null,
        })
        if (res.data?.register) onSuccess()
      })}
    >
      <FieldGroup>
        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{describeError(t, error)}</AlertDescription>
          </Alert>
        ) : null}
        <TextField control={form.control} name="name" label={t.auth.yourName} autoComplete="name" required />
        <TextField control={form.control} name="businessName" label={t.auth.businessName} placeholder={t.auth.businessNamePlaceholder} required />
        <TextField control={form.control} name="phone" type="tel" inputMode="tel" label={t.common.phone} placeholder="01XXXXXXXXX" autoComplete="tel" />
        <TextField control={form.control} name="email" type="email" label={t.common.email} autoComplete="email" />
        <TextField control={form.control} name="password" type="password" label={t.auth.password} autoComplete="new-password" required />
        <Button type="submit" size="lg" className="w-full" disabled={fetching}>
          {fetching ? <Loader2Icon className="animate-spin" /> : null}
          {fetching ? t.auth.registering : t.auth.register}
        </Button>
      </FieldGroup>
    </form>
  )
}
