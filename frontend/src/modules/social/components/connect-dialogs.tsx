"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Loader2Icon, ShieldCheckIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { PlatformIcon, ResponsiveDialog, TextField } from "@/components/shared"
import type { SocialPlatform } from "@/gql/graphql"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useConnectTelegram, useConnectWhatsApp, useStartMetaConnect } from "../hooks/use-social"

function TelegramDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [connect, busy] = useConnectTelegram()
  const schema = useMemo(() => z.object({ botToken: z.string().trim().regex(/^\d+:[\w-]{20,}$/, t.errors.required) }), [t])
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { botToken: "" } })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.social.telegramTitle}
      description={t.social.telegramHint}
      footer={
        <Button type="submit" form="tg-connect" disabled={busy}>
          {t.social.connect(enumLabel(t, "Platform", "TELEGRAM"))}
        </Button>
      }
    >
      <form
        id="tg-connect"
        noValidate
        autoComplete="off"
        onSubmit={form.handleSubmit(async (values) => {
          if (await connect({ input: values })) {
            form.reset()
            onOpenChange(false)
          }
        })}
      >
        <FieldGroup>
          <TextField control={form.control} name="botToken" type="password" label={t.social.botToken} placeholder="123456:ABC-DEF…" required />
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <ShieldCheckIcon className="size-3.5 text-success" /> {t.social.secretHint}
          </p>
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

function WhatsAppDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [connect, busy] = useConnectWhatsApp()
  const schema = useMemo(
    () =>
      z.object({
        displayName: v.requiredString(t),
        phoneNumberId: v.requiredString(t),
        businessAccountId: v.requiredString(t),
        accessToken: v.requiredString(t),
      }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { displayName: "", phoneNumberId: "", businessAccountId: "", accessToken: "" },
  })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.social.whatsappTitle}
      description={t.social.whatsappHint}
      footer={
        <Button type="submit" form="wa-connect" disabled={busy}>
          {t.social.connect(enumLabel(t, "Platform", "WHATSAPP"))}
        </Button>
      }
    >
      <form
        id="wa-connect"
        noValidate
        autoComplete="off"
        onSubmit={form.handleSubmit(async (values) => {
          if (await connect({ input: values })) {
            form.reset()
            onOpenChange(false)
          }
        })}
      >
        <FieldGroup>
          <TextField control={form.control} name="displayName" label={t.social.displayName} required />
          <TextField control={form.control} name="phoneNumberId" label={t.social.phoneNumberId} inputMode="numeric" required />
          <TextField control={form.control} name="businessAccountId" label={t.social.businessAccountId} inputMode="numeric" required />
          <TextField control={form.control} name="accessToken" type="password" label={t.social.accessToken} required />
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <ShieldCheckIcon className="size-3.5 text-success" /> {t.social.secretHint}
          </p>
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

/** One big button per platform. FB/IG go through Meta OAuth; WhatsApp/Telegram take tokens. */
export function ConnectButtons() {
  const t = useT()
  const [startMeta, starting] = useStartMetaConnect()
  const [pending, setPending] = useState<SocialPlatform | null>(null)
  const [tgOpen, setTgOpen] = useState(false)
  const [waOpen, setWaOpen] = useState(false)

  async function connectMeta(platform: SocialPlatform) {
    setPending(platform)
    const res = await startMeta({ platform })
    if (res?.startMetaConnect.url) {
      window.location.assign(res.startMetaConnect.url)
    } else {
      setPending(null)
    }
  }

  const buttons: { platform: SocialPlatform; onClick: () => void }[] = [
    { platform: "FACEBOOK", onClick: () => void connectMeta("FACEBOOK") },
    { platform: "INSTAGRAM", onClick: () => void connectMeta("INSTAGRAM") },
    { platform: "WHATSAPP", onClick: () => setWaOpen(true) },
    { platform: "TELEGRAM", onClick: () => setTgOpen(true) },
  ]

  return (
    <RoleGate roles={["OWNER", "ADMIN"]}>
      <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
        {buttons.map(({ platform, onClick }) => (
          <Button key={platform} variant="outline" className="h-auto justify-start gap-3 py-3" disabled={starting && pending === platform} onClick={onClick}>
            {starting && pending === platform ? <Loader2Icon className="animate-spin" /> : <PlatformIcon platform={platform} className="size-8" />}
            <span className="text-left text-sm font-medium whitespace-normal">{t.social.connect(enumLabel(t, "Platform", platform))}</span>
          </Button>
        ))}
      </div>
      <TelegramDialog open={tgOpen} onOpenChange={setTgOpen} />
      <WhatsAppDialog open={waOpen} onOpenChange={setWaOpen} />
    </RoleGate>
  )
}
