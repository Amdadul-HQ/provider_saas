"use client"

import { useEffect, useRef, useState } from "react"
import { Loader2Icon, RefreshCwIcon, Share2Icon, UnplugIcon } from "lucide-react"
import { toast } from "sonner"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { ConfirmDialog, DateText, EmptyState, ErrorState, ListSkeleton, PlatformIcon, StatusBadge, ToneBadge, ErrorDetails } from "@/components/shared"
import type { SocialAccountFieldsFragment } from "@/gql/graphql"
import { RoleGate } from "@/lib/auth"
import { useT } from "@/i18n"
import { useCompleteMetaConnect, useDisconnectSocialAccount, useSocialAccounts, useSyncSocialAccount } from "../hooks/use-social"

function AccountCard({ account, onDisconnect }: { account: SocialAccountFieldsFragment; onDisconnect: () => void }) {
  const t = useT()
  const [sync, syncing] = useSyncSocialAccount()
  return (
    <div className="flex h-full flex-col gap-3 rounded-2xl border border-border/80 bg-card p-4 shadow-xs">
      <div className="flex items-start gap-3">
        <div className="relative">
          <Avatar className="size-11">
            {account.avatarUrl ? <AvatarImage src={account.avatarUrl} alt="" /> : null}
            <AvatarFallback>{account.name.slice(0, 2)}</AvatarFallback>
          </Avatar>
          <PlatformIcon platform={account.platform} className="absolute -right-1 -bottom-1 size-5 ring-2 ring-card" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">{account.name}</p>
          <p className="truncate text-xs text-muted-foreground">{account.username ?? account.externalId}</p>
        </div>
        <StatusBadge enum="SocialAccountStatus" value={account.status} />
      </div>
      <div className="flex flex-wrap gap-1.5">
        <ToneBadge tone={account.webhookVerified ? "success" : "warning"}>{account.webhookVerified ? t.social.webhookOk : t.social.webhookPending}</ToneBadge>
        <ToneBadge tone="primary">{t.social.openConversations(account.openConversationCount)}</ToneBadge>
      </div>
      {account.lastError ? (
        <div className="rounded-lg bg-destructive/10 p-2 text-xs text-destructive" role="alert">
          {t.social.lastErrorGeneric}
          <ErrorDetails error={account.lastError} />
        </div>
      ) : null}
      <dl className="grid grid-cols-2 gap-2 text-xs">
        <div>
          <dt className="text-muted-foreground">{t.social.lastSynced}</dt>
          <dd>
            <DateText value={account.lastSyncedAt} relative />
          </dd>
        </div>
        <div>
          <dt className="text-muted-foreground">{t.social.tokenExpires}</dt>
          <dd>
            <DateText value={account.tokenExpiresAt} />
          </dd>
        </div>
      </dl>
      <div className="mt-auto flex gap-2">
        <Button size="sm" variant="outline" disabled={syncing} onClick={() => void sync({ id: account.id })}>
          {syncing ? <Loader2Icon className="animate-spin" /> : <RefreshCwIcon />} {t.social.sync}
        </Button>
        <RoleGate roles={["OWNER", "ADMIN"]}>
          <Button size="sm" variant="ghost" className="text-destructive" onClick={onDisconnect}>
            <UnplugIcon /> {t.social.disconnect}
          </Button>
        </RoleGate>
      </div>
    </div>
  )
}

export function SocialAccountList() {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useSocialAccounts()
  const [disconnect, disconnecting] = useDisconnectSocialAccount()
  const [target, setTarget] = useState<SocialAccountFieldsFragment | null>(null)
  const accounts = data?.socialAccounts

  if (!accounts) {
    if (error) return <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <ListSkeleton rows={3} /> : null
  }
  if (accounts.length === 0) return <EmptyState icon={Share2Icon} title={t.social.empty} description={t.social.emptyHint} />
  return (
    <>
      <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {accounts.map((a) => (
          <li key={a.id}>
            <AccountCard account={a} onDisconnect={() => setTarget(a)} />
          </li>
        ))}
      </ul>
      <ConfirmDialog
        open={target !== null}
        onOpenChange={(o) => !o && setTarget(null)}
        title={t.social.disconnect}
        description={t.social.disconnectConfirm}
        destructive
        loading={disconnecting}
        onConfirm={async () => {
          if (target && (await disconnect({ id: target.id }))) setTarget(null)
        }}
      />
    </>
  )
}

/** Finishes Meta OAuth when the provider redirects back with ?code=&state=. Renders nothing. */
export function MetaConnectCallback({ code, state, onDone }: { code: string | null; state: string | null; onDone: () => void }) {
  const t = useT()
  const [complete] = useCompleteMetaConnect()
  const ran = useRef(false)
  useEffect(() => {
    if (!code || !state || ran.current) return
    ran.current = true
    const id = toast.loading(t.social.completing)
    void complete({ code, state }).then((res) => {
      toast.dismiss(id)
      if (res) toast.success(t.social.completed(res.completeMetaConnect.length))
      onDone()
    })
  }, [code, state, complete, onDone, t])
  return null
}
