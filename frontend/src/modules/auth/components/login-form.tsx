"use client"

import { useMemo, useState } from "react"
import { Controller, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { AtSignIcon, CheckIcon, Loader2Icon, MailIcon, SmartphoneIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { ErrorDetails, PasswordField } from "@/components/shared"
import { describeError } from "@/lib/graphql/errors"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useLogin } from "../hooks/use-auth"

type Kind = "phone" | "email" | null

/** Phone if it is only digits/+/spaces/dashes, email once it has an @. */
export function identifierKind(value: string): Kind {
  const v = value.trim()
  if (!v) return null
  if (v.includes("@")) return "email"
  if (/^[+\d\s-]+$/.test(v) && /\d/.test(v)) return "phone"
  return null
}

const SUCCESS_PAUSE_MS = 450

export function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const t = useT()
  const { login, fetching, error } = useLogin()
  const [attempt, setAttempt] = useState(0)
  const [done, setDone] = useState(false)
  const schema = useMemo(
    () => z.object({ identifier: z.string().trim().min(3, t.errors.required), password: z.string().min(1, t.errors.required) }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { identifier: "", password: "" } })
  const identifier = useWatch({ control: form.control, name: "identifier" })
  const kind = identifierKind(identifier ?? "")
  const KindIcon = kind === "phone" ? SmartphoneIcon : kind === "email" ? MailIcon : AtSignIcon

  return (
    <form
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        const res = await login(values)
        if (res.data?.login) {
          setDone(true)
          window.setTimeout(onSuccess, SUCCESS_PAUSE_MS)
        } else {
          setAttempt((n) => n + 1)
          form.setFocus("password")
        }
      }, () => setAttempt((n) => n + 1))}
    >
      <FieldGroup key={attempt} className={cn(attempt > 0 && "animate-shake")}>
        {error ? (
          <Alert variant="destructive" className="animate-pop">
            {/* login maps UNAUTHENTICATED to "wrong credentials", never "session expired" (BUG-05) */}
            <AlertDescription>
              {describeError(t, error, "login")}
              <ErrorDetails error={error} />
            </AlertDescription>
          </Alert>
        ) : null}
        <Controller
          control={form.control}
          name="identifier"
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid || undefined}>
              <FieldLabel htmlFor="f-identifier" className="justify-between">
                <span className="flex items-center gap-1">
                  {t.auth.identifier}
                  <span className="text-destructive" aria-hidden>
                    *
                  </span>
                </span>
                {kind ? (
                  <span key={kind} className="animate-pop rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
                    {kind === "phone" ? t.auth.detectedPhone : t.auth.detectedEmail}
                  </span>
                ) : null}
              </FieldLabel>
              <div className="relative">
                <span key={kind ?? "none"} className="animate-pop pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground" aria-hidden>
                  <KindIcon className={cn("size-4 transition-colors", kind && "text-primary")} />
                </span>
                <Input
                  id="f-identifier"
                  autoComplete="username"
                  autoCapitalize="none"
                  spellCheck={false}
                  placeholder={t.auth.identifierPlaceholder}
                  aria-invalid={fieldState.invalid}
                  className="pl-9"
                  {...field}
                />
              </div>
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />
        <PasswordField control={form.control} name="password" label={t.auth.password} autoComplete="current-password" required />
        <Button
          type="submit"
          size="lg"
          className={cn("h-12 w-full rounded-xl text-base transition-[background-color,transform]", done && "bg-success hover:bg-success")}
          disabled={fetching || done}
        >
          {done ? (
            <span className="animate-pop flex items-center gap-2">
              <CheckIcon strokeWidth={3} /> {t.auth.welcomeIn}
            </span>
          ) : fetching ? (
            <>
              <Loader2Icon className="animate-spin" /> {t.auth.loggingIn}
            </>
          ) : (
            t.auth.login
          )}
        </Button>
      </FieldGroup>
    </form>
  )
}
