"use client"

import { useEffect, useState, type ReactNode } from "react"
import { Controller, type Control, type FieldPath, type FieldValues } from "react-hook-form"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import { Checkbox } from "@/components/ui/checkbox"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { CheckIcon, EyeIcon, EyeOffIcon, TriangleAlertIcon } from "lucide-react"
import { currencySymbol, fromMinor, toMinor } from "@/lib/money"
import { passwordChecks, passwordScore } from "@/lib/password"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

type BaseProps<T extends FieldValues> = {
  control: Control<T>
  name: FieldPath<T>
  label: ReactNode
  description?: ReactNode
  required?: boolean
  className?: string
  disabled?: boolean
}

function Label({ htmlFor, label, required }: { htmlFor: string; label: ReactNode; required?: boolean }) {
  return (
    <FieldLabel htmlFor={htmlFor}>
      {label}
      {required ? <span className="text-destructive" aria-hidden>*</span> : null}
    </FieldLabel>
  )
}

function fieldId(name: string) {
  return `f-${name.replace(/[^a-zA-Z0-9_-]/g, "-")}`
}

export function TextField<T extends FieldValues>({
  control,
  name,
  label,
  description,
  required,
  className,
  disabled,
  type = "text",
  placeholder,
  autoComplete,
  inputMode,
}: BaseProps<T> & {
  type?: "text" | "email" | "tel" | "password" | "url" | "search" | "date" | "datetime-local"
  placeholder?: string
  autoComplete?: string
  inputMode?: React.HTMLAttributes<HTMLInputElement>["inputMode"]
}) {
  const id = fieldId(name)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid || undefined} className={className}>
          <Label htmlFor={id} label={label} required={required} />
          <Input
            id={id}
            type={type}
            placeholder={placeholder}
            autoComplete={autoComplete}
            inputMode={inputMode}
            disabled={disabled}
            aria-invalid={fieldState.invalid}
            {...field}
            value={(field.value as string | null | undefined) ?? ""}
          />
          {description ? <FieldDescription>{description}</FieldDescription> : null}
          <FieldError errors={[fieldState.error]} />
        </Field>
      )}
    />
  )
}

export function TextareaField<T extends FieldValues>({
  control,
  name,
  label,
  description,
  required,
  className,
  disabled,
  placeholder,
  rows = 3,
}: BaseProps<T> & { placeholder?: string; rows?: number }) {
  const id = fieldId(name)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid || undefined} className={className}>
          <Label htmlFor={id} label={label} required={required} />
          <Textarea
            id={id}
            rows={rows}
            placeholder={placeholder}
            disabled={disabled}
            aria-invalid={fieldState.invalid}
            {...field}
            value={(field.value as string | null | undefined) ?? ""}
          />
          {description ? <FieldDescription>{description}</FieldDescription> : null}
          <FieldError errors={[fieldState.error]} />
        </Field>
      )}
    />
  )
}

/** Integer/decimal field; empty input → `null`. */
export function NumberField<T extends FieldValues>({
  control,
  name,
  label,
  description,
  required,
  className,
  disabled,
  placeholder,
  min,
  max,
  step,
}: BaseProps<T> & { placeholder?: string; min?: number; max?: number; step?: number }) {
  const id = fieldId(name)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid || undefined} className={className}>
          <Label htmlFor={id} label={label} required={required} />
          <Input
            id={id}
            type="number"
            inputMode="numeric"
            min={min}
            max={max}
            step={step}
            placeholder={placeholder}
            disabled={disabled}
            aria-invalid={fieldState.invalid}
            name={field.name}
            ref={field.ref}
            onBlur={field.onBlur}
            value={field.value === null || field.value === undefined ? "" : String(field.value)}
            onChange={(e) => field.onChange(e.target.value === "" ? null : Number(e.target.value))}
          />
          {description ? <FieldDescription>{description}</FieldDescription> : null}
          <FieldError errors={[fieldState.error]} />
        </Field>
      )}
    />
  )
}

/** Shows taka to the user, stores integer paisa (Money scalar) in the form. */
export function MoneyInput({
  id,
  value,
  onChange,
  onBlur,
  currency = "BDT",
  disabled,
  invalid,
  placeholder,
}: {
  id?: string
  value: number | null | undefined
  onChange: (minor: number | null) => void
  onBlur?: () => void
  currency?: string
  disabled?: boolean
  invalid?: boolean
  placeholder?: string
}) {
  const [text, setText] = useState(value === null || value === undefined ? "" : String(fromMinor(value)))
  useEffect(() => {
    const parsed = text === "" ? null : toMinor(text)
    if (parsed !== (value ?? null)) setText(value === null || value === undefined ? "" : String(fromMinor(value)))
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only react to external value changes
  }, [value])
  return (
    <div className="relative">
      <span className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-sm text-muted-foreground">
        {currencySymbol(currency)}
      </span>
      <Input
        id={id}
        inputMode="decimal"
        className="pl-8 tabular"
        disabled={disabled}
        aria-invalid={invalid}
        placeholder={placeholder ?? "0"}
        value={text}
        onBlur={onBlur}
        onChange={(e) => {
          const next = e.target.value.replace(/[^\d.]/g, "")
          setText(next)
          onChange(next === "" ? null : toMinor(next))
        }}
      />
    </div>
  )
}

export function MoneyField<T extends FieldValues>({
  control,
  name,
  label,
  description,
  required,
  className,
  disabled,
  currency,
}: BaseProps<T> & { currency?: string }) {
  const id = fieldId(name)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid || undefined} className={className}>
          <Label htmlFor={id} label={label} required={required} />
          <MoneyInput
            id={id}
            value={field.value as number | null}
            onChange={field.onChange}
            onBlur={field.onBlur}
            currency={currency}
            disabled={disabled}
            invalid={fieldState.invalid}
          />
          {description ? <FieldDescription>{description}</FieldDescription> : null}
          <FieldError errors={[fieldState.error]} />
        </Field>
      )}
    />
  )
}

export type Option = { value: string; label: ReactNode }

/** Radix Select can't hold "" — we map an empty choice to this sentinel. */
const EMPTY = "__none__"

export function SelectField<T extends FieldValues>({
  control,
  name,
  label,
  description,
  required,
  className,
  disabled,
  options,
  placeholder,
  emptyLabel,
}: BaseProps<T> & { options: Option[]; placeholder?: string; emptyLabel?: string }) {
  const id = fieldId(name)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid || undefined} className={className}>
          <Label htmlFor={id} label={label} required={required} />
          <Select
            disabled={disabled}
            value={(field.value as string | null | undefined) ?? (emptyLabel ? EMPTY : "")}
            onValueChange={(v) => field.onChange(v === EMPTY ? null : v)}
          >
            <SelectTrigger id={id} className="w-full" aria-invalid={fieldState.invalid} onBlur={field.onBlur}>
              <SelectValue placeholder={placeholder} />
            </SelectTrigger>
            <SelectContent>
              {emptyLabel ? <SelectItem value={EMPTY}>{emptyLabel}</SelectItem> : null}
              {options.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {description ? <FieldDescription>{description}</FieldDescription> : null}
          <FieldError errors={[fieldState.error]} />
        </Field>
      )}
    />
  )
}

export function SwitchField<T extends FieldValues>({ control, name, label, description, className, disabled }: BaseProps<T>) {
  const id = fieldId(name)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => (
        <Field orientation="horizontal" className={cn("min-h-11 justify-between rounded-lg border px-3 py-2", className)}>
          <div className="space-y-0.5">
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            {description ? <FieldDescription>{description}</FieldDescription> : null}
          </div>
          <Switch id={id} disabled={disabled} checked={Boolean(field.value)} onCheckedChange={field.onChange} />
        </Field>
      )}
    />
  )
}

export function CheckboxField<T extends FieldValues>({ control, name, label, description, className, disabled }: BaseProps<T>) {
  const id = fieldId(name)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => (
        <Field orientation="horizontal" className={cn("min-h-10", className)}>
          <Checkbox id={id} disabled={disabled} checked={Boolean(field.value)} onCheckedChange={(v) => field.onChange(v === true)} />
          <div className="space-y-0.5">
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            {description ? <FieldDescription>{description}</FieldDescription> : null}
          </div>
        </Field>
      )}
    />
  )
}

/**
 * Password input with show/hide toggle and a Caps Lock warning. `showStrength` adds a 3-step meter
 * and a live rule checklist (sign-up, new password). Strength is guidance only; the schema decides.
 */
export function PasswordField<T extends FieldValues>({
  control,
  name,
  label,
  description,
  required,
  className,
  disabled,
  autoComplete = "current-password",
  showStrength = false,
}: BaseProps<T> & { autoComplete?: string; showStrength?: boolean }) {
  const id = fieldId(name)
  const t = useT()
  const [visible, setVisible] = useState(false)
  const [caps, setCaps] = useState(false)
  const onKey = (e: React.KeyboardEvent<HTMLInputElement>) => setCaps(e.getModifierState?.("CapsLock") ?? false)
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => {
        const value = (field.value as string | null | undefined) ?? ""
        const score = passwordScore(value)
        const checks = passwordChecks(value)
        return (
          <Field data-invalid={fieldState.invalid || undefined} className={className}>
            <Label htmlFor={id} label={label} required={required} />
            <div className="relative">
              <Input
                id={id}
                type={visible ? "text" : "password"}
                autoComplete={autoComplete}
                disabled={disabled}
                aria-invalid={fieldState.invalid}
                aria-describedby={showStrength ? `${id}-strength` : undefined}
                className="pr-11"
                onKeyDown={onKey}
                onKeyUp={onKey}
                {...field}
                onBlur={() => {
                  setCaps(false)
                  field.onBlur()
                }}
                value={value}
              />
              <button
                type="button"
                onClick={() => setVisible((v) => !v)}
                aria-label={visible ? t.auth.hidePassword : t.auth.showPassword}
                aria-pressed={visible}
                disabled={disabled}
                className="absolute top-1/2 right-1.5 flex size-8 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                <span key={String(visible)} className="animate-pop">
                  {visible ? <EyeOffIcon className="size-4" aria-hidden /> : <EyeIcon className="size-4" aria-hidden />}
                </span>
              </button>
            </div>
            {caps ? (
              <p className="animate-pop flex items-center gap-1.5 text-xs font-medium text-warning-foreground dark:text-warning" role="status">
                <TriangleAlertIcon className="size-3.5" aria-hidden /> {t.auth.capsLock}
              </p>
            ) : null}
            {showStrength && value ? (
              <div id={`${id}-strength`} className="animate-pop space-y-2" aria-live="polite">
                <div className="flex items-center gap-2">
                  <div className="grid flex-1 grid-cols-3 gap-1" aria-hidden>
                    {[1, 2, 3].map((n) => (
                      <span
                        key={n}
                        className={cn(
                          "h-1.5 rounded-full bg-muted transition-colors duration-(--dur)",
                          score >= n && (score === 1 ? "bg-destructive" : score === 2 ? "bg-warning" : "bg-success"),
                        )}
                      />
                    ))}
                  </div>
                  <span
                    className={cn(
                      "min-w-16 text-right text-xs font-medium",
                      score === 1 ? "text-destructive" : score === 2 ? "text-warning-foreground dark:text-warning" : "text-success",
                    )}
                  >
                    <span className="sr-only">{t.auth.strengthLabel}: </span>
                    {score === 1 ? t.auth.strength.weak : score === 2 ? t.auth.strength.fair : t.auth.strength.strong}
                  </span>
                </div>
                <ul className="grid gap-1 text-xs sm:grid-cols-2">
                  {(
                    [
                      [checks.length, t.auth.ruleLength],
                      [checks.letterDigit, t.auth.ruleLetterDigit],
                      [checks.extra, t.auth.ruleExtra],
                    ] as const
                  ).map(([ok, text]) => (
                    <li key={text} className={cn("flex items-center gap-1.5 transition-colors duration-(--dur)", ok ? "text-success" : "text-muted-foreground")}>
                      <span
                        className={cn(
                          "flex size-4 shrink-0 items-center justify-center rounded-full transition-[background-color,transform] duration-(--dur) ease-(--ease-spring)",
                          ok ? "scale-100 bg-success text-success-foreground" : "scale-90 bg-muted",
                        )}
                        aria-hidden
                      >
                        {ok ? <CheckIcon className="size-2.5" strokeWidth={3} /> : null}
                      </span>
                      {text}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {description ? <FieldDescription>{description}</FieldDescription> : null}
            <FieldError errors={[fieldState.error]} />
          </Field>
        )
      }}
    />
  )
}
