"use client"

import { useMemo } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { TextField } from "@/components/shared"
import { useT } from "@/i18n"
import { useChangePassword } from "../hooks/use-auth"

export function ChangePasswordForm() {
  const t = useT()
  const [changePassword, fetching] = useChangePassword()
  const schema = useMemo(
    () => z.object({ currentPassword: z.string().min(1, t.errors.required), newPassword: z.string().min(8, t.errors.minLength(8)) }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { currentPassword: "", newPassword: "" } })
  return (
    <form
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        if (await changePassword(values)) form.reset()
      })}
    >
      <FieldGroup>
        <div className="grid gap-4 sm:grid-cols-2 xl:max-w-3xl">
          <TextField control={form.control} name="currentPassword" type="password" label={t.auth.currentPassword} autoComplete="current-password" required />
          <TextField control={form.control} name="newPassword" type="password" label={t.auth.newPassword} autoComplete="new-password" required />
        </div>
        <Button type="submit" disabled={fetching} className="w-fit">
          {t.auth.changePassword}
        </Button>
      </FieldGroup>
    </form>
  )
}
