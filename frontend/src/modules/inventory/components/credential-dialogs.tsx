"use client"

import { useEffect, useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { EyeIcon, ShieldAlertIcon } from "lucide-react"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { CopyButton, ResponsiveDialog, TextField } from "@/components/shared"
import type { RevealMasterAccountCredentialMutation } from "@/gql/graphql"
import { v } from "@/lib/validation"
import { useT } from "@/i18n"
import { useRevealCredential, useSetCredential } from "../hooks/use-inventory"

const AUTO_HIDE_SECONDS = 30

type Revealed = RevealMasterAccountCredentialMutation["revealMasterAccountCredential"]

/**
 * Two-step reveal: audit warning first, then the (audit-logged) mutation. Plaintext lives only in
 * component state, is wiped on close and auto-hides after 30 s.
 */
export function CredentialRevealDialog({
  accountId,
  accountLabel,
  open,
  onOpenChange,
}: {
  accountId: string
  accountLabel: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const t = useT()
  const [reveal, revealing] = useRevealCredential()
  const [secret, setSecret] = useState<Revealed | null>(null)
  const [left, setLeft] = useState(AUTO_HIDE_SECONDS)

  useEffect(() => {
    if (!secret) return
    const id = setInterval(() => {
      setLeft((s) => {
        if (s <= 1) {
          setSecret(null)
          onOpenChange(false)
          return AUTO_HIDE_SECONDS
        }
        return s - 1
      })
    }, 1000)
    return () => clearInterval(id)
  }, [secret, onOpenChange])

  const close = (o: boolean) => {
    if (!o) {
      setSecret(null)
      setLeft(AUTO_HIDE_SECONDS)
    }
    onOpenChange(o)
  }

  return (
    <ResponsiveDialog open={open} onOpenChange={close} title={`${t.inventory.revealTitle} · ${accountLabel}`}>
      {!secret ? (
        <div className="space-y-4">
          <Alert className="border-warning/50 bg-warning/10">
            <ShieldAlertIcon className="text-warning-foreground dark:text-warning" />
            <AlertTitle>{t.inventory.revealTitle}</AlertTitle>
            <AlertDescription>{t.inventory.revealWarning}</AlertDescription>
          </Alert>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => close(false)}>
              {t.common.cancel}
            </Button>
            <Button
              disabled={revealing}
              onClick={async () => {
                const res = await reveal({ id: accountId })
                if (res) setSecret(res.revealMasterAccountCredential)
              }}
            >
              <EyeIcon /> {t.inventory.revealConfirm}
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {(
            [
              [t.inventory.username, secret.username],
              [t.inventory.password, secret.password],
            ] as const
          ).map(([label, value]) => (
            <div key={label} className="flex items-center justify-between gap-2 rounded-lg border bg-muted/50 p-3">
              <div className="min-w-0">
                <p className="text-xs text-muted-foreground">{label}</p>
                <p className="font-mono text-sm break-all select-all">{value}</p>
              </div>
              <CopyButton value={value} size="icon-sm" />
            </div>
          ))}
          {secret.extra ? <pre className="overflow-x-auto rounded-lg bg-muted p-3 text-xs">{JSON.stringify(secret.extra, null, 2)}</pre> : null}
          <p className="text-center text-xs text-muted-foreground">{t.inventory.revealAutoHide(left)}</p>
        </div>
      )}
    </ResponsiveDialog>
  )
}

export function SetCredentialDialog({ accountId, open, onOpenChange }: { accountId: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [save, saving] = useSetCredential()
  const schema = useMemo(() => z.object({ username: v.requiredString(t), password: v.requiredString(t) }), [t])
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { username: "", password: "" } })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={(o) => {
        if (!o) form.reset()
        onOpenChange(o)
      }}
      title={t.inventory.setCredential}
      description={t.inventory.credentialHint}
      footer={
        <Button type="submit" form="set-credential" disabled={saving}>
          {t.common.save}
        </Button>
      }
    >
      <form
        id="set-credential"
        noValidate
        autoComplete="off"
        onSubmit={form.handleSubmit(async (values) => {
          if (await save({ id: accountId, ...values })) {
            form.reset()
            onOpenChange(false)
          }
        })}
      >
        <FieldGroup>
          <TextField control={form.control} name="username" label={t.inventory.username} autoComplete="off" required />
          <TextField control={form.control} name="password" type="password" label={t.inventory.password} autoComplete="new-password" required />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}
