"use client"

import { useMemo } from "react"
import Link from "next/link"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Loader2Icon, MailWarningIcon } from "lucide-react"
import { toast } from "sonner"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { EmptyState, ErrorDetails, ErrorState, ListSkeleton, TextField } from "@/components/shared"
import type { InviteQuery } from "@/gql/graphql"
import { describeError } from "@/lib/graphql/errors"
import { enumLabel, useFormat, useT } from "@/i18n"
import { useAcceptInvite, useInvitePreview } from "../hooks/use-auth"

type Preview = NonNullable<InviteQuery["invite"]>

function AcceptInviteForm({ token, invite, onAccepted }: { token: string; invite: Preview; onAccepted: () => void }) {
  const t = useT()
  const { accept, fetching, error } = useAcceptInvite()
  const existing = invite.userExists
  const schema = useMemo(
    () =>
      z.object({
        name: z.string().trim().min(1, t.errors.required),
        // existing users prove ownership with their current password (any length); new users choose one
        password: existing ? z.string().min(1, t.errors.required) : z.string().min(8, t.errors.minLength(8)),
      }),
    [t, existing],
  )
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { name: invite.inviteeName, password: "" } })
  return (
    <form
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        const res = await accept({ token, password: values.password, name: existing ? null : values.name })
        if (res.data?.acceptInvite) {
          toast.success(t.auth.inviteAccepted)
          onAccepted()
        }
      })}
    >
      <FieldGroup>
        {error ? (
          <Alert variant="destructive">
            <AlertDescription>
              {describeError(t, error, "invite")}
              <ErrorDetails error={error} />
            </AlertDescription>
          </Alert>
        ) : null}
        {existing ? <p className="rounded-lg bg-info/10 p-3 text-sm text-foreground">{t.auth.inviteExistingHint}</p> : null}
        <TextField control={form.control} name="name" label={t.auth.yourName} autoComplete="name" required disabled={existing} />
        <TextField
          control={form.control}
          name="password"
          type="password"
          label={existing ? t.auth.confirmExistingPassword : t.auth.newPassword8}
          autoComplete={existing ? "current-password" : "new-password"}
          required
        />
        <Button type="submit" size="lg" className="w-full" disabled={fetching}>
          {fetching ? <Loader2Icon className="animate-spin" /> : null}
          {fetching ? t.auth.accepting : t.auth.acceptInvite}
        </Button>
      </FieldGroup>
    </form>
  )
}

/** Public `/invite/[token]` screen (BUG-02): preview the invite, then set or confirm a password. */
export function AcceptInviteView({ token, onAccepted }: { token: string; onAccepted: () => void }) {
  const t = useT()
  const f = useFormat()
  const [{ data, fetching, error }, refetch] = useInvitePreview(token)
  if (!data) {
    if (error) return <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <ListSkeleton rows={3} /> : null
  }
  const invite = data.invite
  if (!invite) {
    return (
      <EmptyState
        icon={MailWarningIcon}
        title={t.auth.inviteInvalid}
        description={t.auth.inviteInvalidHint}
        action={
          <Button asChild variant="outline">
            <Link href="/login">{t.auth.goToLogin}</Link>
          </Button>
        }
      />
    )
  }
  const hint = invite.emailHint ?? invite.phoneHint
  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="font-heading text-2xl font-semibold break-words">{t.auth.inviteTitle(invite.tenantName)}</h1>
        <p className="text-sm text-muted-foreground">{t.auth.inviteSubtitle(enumLabel(t, "Role", invite.role))}</p>
      </div>
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 rounded-xl border bg-muted/40 p-3 text-sm">
        {hint ? (
          <>
            <dt className="text-muted-foreground">{t.auth.inviteSentTo}</dt>
            <dd className="truncate font-mono">{hint}</dd>
          </>
        ) : null}
        <dt className="text-muted-foreground">{t.auth.inviteExpires}</dt>
        <dd>
          <time dateTime={invite.expiresAt}>{f.dateTime(invite.expiresAt)}</time>
        </dd>
      </dl>
      <AcceptInviteForm token={token} invite={invite} onAccepted={onAccepted} />
    </div>
  )
}
