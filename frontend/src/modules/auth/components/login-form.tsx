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
import { useT } from "@/i18n"
import { useLogin } from "../hooks/use-auth"

export function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const t = useT()
  const { login, fetching, error } = useLogin()
  const schema = useMemo(
    () => z.object({ identifier: z.string().trim().min(3, t.errors.required), password: z.string().min(1, t.errors.required) }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { identifier: "", password: "" } })

  return (
    <form
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        const res = await login(values)
        if (res.data?.login) onSuccess()
      })}
    >
      <FieldGroup>
        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{describeError(t, error)}</AlertDescription>
          </Alert>
        ) : null}
        <TextField control={form.control} name="identifier" label={t.auth.identifier} placeholder={t.auth.identifierPlaceholder} autoComplete="username" required />
        <TextField control={form.control} name="password" type="password" label={t.auth.password} autoComplete="current-password" required />
        <Button type="submit" size="lg" className="w-full" disabled={fetching}>
          {fetching ? <Loader2Icon className="animate-spin" /> : null}
          {fetching ? t.auth.loggingIn : t.auth.login}
        </Button>
      </FieldGroup>
    </form>
  )
}
