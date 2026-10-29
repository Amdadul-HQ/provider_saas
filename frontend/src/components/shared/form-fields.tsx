"use client"

import { useEffect, useState, type ReactNode } from "react"
import { Controller, type Control, type FieldPath, type FieldValues } from "react-hook-form"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import { Checkbox } from "@/components/ui/checkbox"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { currencySymbol, fromMinor, toMinor } from "@/lib/money"
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
