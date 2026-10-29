"use client"

import { useMemo, useState } from "react"
import Link from "next/link"
import { useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon, GlobeIcon, Loader2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { PasswordField, TextField } from "@/components/shared"
import { describeError } from "@/lib/graphql/use-action"
import { errorField } from "@/lib/graphql/errors"
import { APP_URL } from "@/lib/config"
import { previewSlug } from "@/lib/slug"
import { normalizePhone, v } from "@/lib/validation"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useRegister } from "../hooks/use-auth"

const STEP_FIELDS = [["name", "businessName"], ["phone", "email", "password"]] as const
const SUCCESS_PAUSE_MS = 650

function appHost() {
  try {
    return new URL(APP_URL).host
  } catch {
    return "sellerdesk"
  }
}

/** Live preview of the store link the API will derive from the business name. */
function StoreLinkPreview({ businessName }: { businessName: string }) {
  const t = useT()
  const slug = previewSlug(businessName)
  if (!slug) return null
  const initials = businessName.trim().split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? "").join("")
  return (
    <div className="animate-pop flex items-center gap-3 rounded-2xl bg-secondary/70 p-3">
      <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary font-heading text-sm font-bold text-primary-foreground">{initials}</span>
      <span className="min-w-0">
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <GlobeIcon className="size-3" aria-hidden /> {t.auth.storeLink}
        </span>
        <span className="block truncate text-sm font-semibold">
          {appHost()}/s/<span className="text-primary">{slug}</span>
        </span>
        <span className="block text-[11px] text-muted-foreground">{t.auth.storeLinkHint}</span>
      </span>
    </div>
  )
}

/** Two short steps (you + business, then login details) so a phone screen never shows five fields at once. */
export function RegisterForm({ onSuccess }: { onSuccess: () => void }) {
  const t = useT()
  const { register, fetching, error } = useRegister()
  const [step, setStep] = useState<0 | 1>(0)
  const [attempt, setAttempt] = useState(0)
  const [done, setDone] = useState(false)
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
  const businessName = useWatch({ control: form.control, name: "businessName" }) ?? ""

  const next = async () => {
    if (await form.trigger([...STEP_FIELDS[0]], { shouldFocus: true })) {
      setStep(1)
      window.setTimeout(() => form.setFocus("phone"), 0)
    } else setAttempt((n) => n + 1)
  }
  const back = () => {
    setStep(0)
    window.setTimeout(() => form.setFocus("businessName"), 0)
  }

  const submit = form.handleSubmit(
    async (values) => {
      const res = await register({
        name: values.name,
        businessName: values.businessName,
        password: values.password,
        phone: values.phone ? normalizePhone(values.phone) : null,
        email: values.email || null,
      })
      if (res.data?.register) {
        setDone(true)
        window.setTimeout(onSuccess, SUCCESS_PAUSE_MS)
      } else setAttempt((n) => n + 1)
    },
    (errors) => {
      setAttempt((n) => n + 1)
      // an error on step 1 fields (e.g. after Back + edit) sends the seller back there
      if (errors.name || errors.businessName) setStep(0)
    },
  )

  const titles = [t.auth.stepAbout, t.auth.stepAccount]
  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        if (step === 0) void next()
        else void submit(e)
      }}
      className="space-y-5"
    >
      <div className="space-y-2">
        <div className="flex items-baseline justify-between gap-2">
          <p className="font-heading font-semibold">{titles[step]}</p>
          <p className="text-xs text-muted-foreground tabular">{t.auth.stepOf(step + 1, 2)}</p>
        </div>
        <div className="grid grid-cols-2 gap-1.5" aria-hidden>
          {[0, 1].map((i) => (
            <span key={i} className="h-1.5 overflow-hidden rounded-full bg-muted">
              <span className={cn("block h-full origin-left rounded-full bg-primary transition-transform duration-(--dur-slow) ease-(--ease-smooth)", step >= i ? "scale-x-100" : "scale-x-0")} />
            </span>
          ))}
        </div>
      </div>

      <FieldGroup key={`${step}-${attempt}`} className={cn(attempt > 0 && "animate-shake", "animate-in fade-in-0 duration-300", step === 1 ? "slide-in-from-right-4" : attempt === 0 && "slide-in-from-left-4")}>
        {error ? (
          <Alert variant="destructive" className="animate-pop">
            {errorField(error) === "account" ? (
              // the phone/email is taken: point to login instead of "check your input" (the API never says which one)
              <AlertDescription className="space-y-2">
                <p>{t.auth.registerAccountExists}</p>
                <Button asChild size="sm" variant="outline" className="w-fit">
                  <Link href="/login">{t.auth.loginInstead}</Link>
                </Button>
              </AlertDescription>
            ) : (
              <AlertDescription>{describeError(t, error)}</AlertDescription>
            )}
          </Alert>
        ) : null}
        {step === 0 ? (
          <>
            <TextField control={form.control} name="name" label={t.auth.yourName} autoComplete="name" required />
            <TextField control={form.control} name="businessName" label={t.auth.businessName} placeholder={t.auth.businessNamePlaceholder} autoComplete="organization" required />
            <StoreLinkPreview businessName={businessName} />
          </>
        ) : (
          <>
            <p className="-mb-2 text-sm text-muted-foreground">{t.auth.stepAccountHint}</p>
            <TextField control={form.control} name="phone" type="tel" inputMode="tel" label={t.common.phone} placeholder="01XXXXXXXXX" autoComplete="tel" />
            <TextField control={form.control} name="email" type="email" label={t.common.email} autoComplete="email" />
            <PasswordField control={form.control} name="password" label={t.auth.password} autoComplete="new-password" required showStrength />
          </>
        )}
      </FieldGroup>

      <div className="flex gap-2">
        {step === 1 ? (
          <Button type="button" variant="outline" size="lg" className="h-12 rounded-xl" onClick={back} disabled={fetching || done} aria-label={t.common.back}>
            <ArrowLeftIcon />
          </Button>
        ) : null}
        <Button
          type="submit"
          size="lg"
          className={cn("h-12 flex-1 rounded-xl text-base", done && "bg-success hover:bg-success")}
          disabled={fetching || done}
        >
          {done ? (
            <span className="animate-pop flex items-center gap-2">
              <CheckIcon strokeWidth={3} /> {t.auth.accountReady}
            </span>
          ) : fetching ? (
            <>
              <Loader2Icon className="animate-spin" /> {t.auth.registering}
            </>
          ) : step === 0 ? (
            <>
              {t.auth.nextStep} <ArrowRightIcon />
            </>
          ) : (
            t.auth.register
          )}
        </Button>
      </div>
    </form>
  )
}
