"use client"

import { useState } from "react"
import { KeyRoundIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ConfirmDialog, CopyButton, OneTimeSecretDialog, SectionCard } from "@/components/shared"
import { RoleGate } from "@/lib/auth"
import { GRAPHQL_URL } from "@/lib/config"
import { useT } from "@/i18n"
import { useRotateSmsWebhookToken } from "../hooks/use-tenant"

function smsEndpoint(): string {
  try {
    return new URL("/webhooks/payments/sms", GRAPHQL_URL).toString()
  } catch {
    return "/webhooks/payments/sms"
  }
}

/** Settings → Integrations: rotate the SMS-forwarder bearer token; the new token is shown once. */
export function SmsTokenCard() {
  const t = useT()
  const [rotate, rotating] = useRotateSmsWebhookToken()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [token, setToken] = useState<string | null>(null)
  const endpoint = smsEndpoint()
  return (
    <SectionCard title={t.settings.smsToken} className="max-w-2xl">
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">{t.settings.smsTokenHint}</p>
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground">{t.settings.smsEndpoint}</p>
          <div className="flex items-center gap-2 rounded-lg border bg-muted/40 p-2">
            <code className="min-w-0 flex-1 font-mono text-xs break-all">{endpoint}</code>
            <CopyButton value={endpoint} size="icon-sm" />
          </div>
          <p className="text-xs text-muted-foreground">{t.settings.smsHeaderHint}</p>
        </div>
        <RoleGate roles={["OWNER", "ADMIN"]}>
          <Button variant="outline" onClick={() => setConfirmOpen(true)} disabled={rotating}>
            <KeyRoundIcon /> {t.settings.rotateSmsToken}
          </Button>
        </RoleGate>
      </div>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t.settings.rotateSmsToken}
        description={t.settings.rotateSmsTokenConfirm}
        confirmLabel={t.settings.rotateSmsToken}
        loading={rotating}
        onConfirm={async () => {
          const res = await rotate({})
          setConfirmOpen(false)
          if (res?.rotateSmsWebhookToken) setToken(res.rotateSmsWebhookToken)
        }}
      />
      <OneTimeSecretDialog value={token} onClose={() => setToken(null)} title={t.settings.smsTokenTitle} warning={t.settings.smsTokenWarning} />
    </SectionCard>
  )
}
