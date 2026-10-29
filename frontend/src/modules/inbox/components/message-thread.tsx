"use client"

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react"
import { AlertCircleIcon, ArrowLeftIcon, CheckCheckIcon, ClockIcon, PaperclipIcon, SendIcon, TagIcon, UserPlusIcon, XIcon, ZapIcon } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { EmptyState, ErrorState, ListSkeleton, PlatformIcon } from "@/components/shared"
import type { ConversationListFieldsFragment, ConversationStatus, MessageFieldsFragment } from "@/gql/graphql"
import { usePoll } from "@/hooks/use-poll"
import { formatTime, formatDate } from "@/lib/dates"
import { CONVERSATION_STATUSES } from "@/lib/enums"
import { useSession } from "@/lib/auth"
import { enumLabel, useLocale, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import {
  useAssignConversation,
  useConversation,
  useConversationLabels,
  useMarkConversationRead,
  useMessageTemplates,
  useSendMessage,
  useSetConversationStatus,
} from "../hooks/use-inbox"
import { conversationTitle } from "./conversation-list"

function MessageBubble({ m }: { m: MessageFieldsFragment }) {
  const t = useT()
  const locale = useLocale()
  const out = m.direction === "OUTBOUND"
  if (m.kind === "SYSTEM") {
    return <p className="mx-auto my-1 max-w-[80%] rounded-full bg-muted px-3 py-1 text-center text-xs text-muted-foreground">{m.body}</p>
  }
  const failed = m.status === "failed" || Boolean(m.error)
  const hasAttachments = Array.isArray(m.attachments) && m.attachments.length > 0
  return (
    <div className={cn("flex", out ? "justify-end" : "justify-start")}>
      <div
        className={cn(
          "max-w-[82%] rounded-2xl px-3.5 py-2 text-sm shadow-xs md:max-w-[70%]",
          out ? "rounded-br-md bg-primary text-primary-foreground" : "rounded-bl-md border bg-card",
          failed && "bg-destructive/10 text-foreground ring-1 ring-destructive/40",
        )}
      >
        {m.kind !== "TEXT" ? (
          <p className="mb-1 flex items-center gap-1 text-xs opacity-80">
            <PaperclipIcon className="size-3" /> {enumLabel(t, "MessageKind", m.kind)}
          </p>
        ) : null}
        {m.body ? <p className="break-words whitespace-pre-wrap">{m.body}</p> : null}
        {hasAttachments && !m.body ? <p className="text-xs italic opacity-80">{t.inbox.attachment}</p> : null}
        <p className={cn("mt-1 flex items-center justify-end gap-1 text-[10px]", out ? "text-primary-foreground/75" : "text-muted-foreground")}>
          {out && m.sentBy ? <span>{m.sentBy.name} ·</span> : null}
          {formatTime(m.sentAt, locale)}
          {out ? failed ? <AlertCircleIcon className="size-3 text-destructive" /> : m.status === "pending" ? <ClockIcon className="size-3" /> : <CheckCheckIcon className="size-3" /> : null}
        </p>
        {failed ? <p className="text-[11px] text-destructive">{m.error ?? t.inbox.messageFailed}</p> : null}
      </div>
    </div>
  )
}

function QuickReplies({ onPick }: { onPick: (body: string, templateId: string) => void }) {
  const t = useT()
  const [open, setOpen] = useState(false)
  const [{ data, fetching }] = useMessageTemplates()
  const templates = (data?.messageTemplates ?? []).filter((tpl) => tpl.isActive)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="icon" aria-label={t.inbox.quickReplies}>
          <ZapIcon />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" side="top" className="w-[min(22rem,calc(100vw-1rem))] p-0">
        <p className="border-b px-3 py-2 text-sm font-semibold">{t.inbox.quickReplies}</p>
        <ul className="max-h-72 overflow-y-auto">
          {templates.length === 0 ? (
            <li className="p-3 text-sm text-muted-foreground">{fetching ? t.common.loading : t.inbox.noTemplates}</li>
          ) : (
            templates.map((tpl) => (
              <li key={tpl.id}>
                <button
                  type="button"
                  className="block w-full px-3 py-2 text-left hover:bg-muted"
                  onClick={() => {
                    onPick(tpl.body, tpl.id)
                    setOpen(false)
                  }}
                >
                  <p className="text-sm font-medium">{tpl.name}</p>
                  <p className="line-clamp-2 text-xs text-muted-foreground">{tpl.body}</p>
                </button>
              </li>
            ))
          )}
        </ul>
      </PopoverContent>
    </Popover>
  )
}

function Composer({ conversationId, canReply }: { conversationId: string; canReply: boolean }) {
  const t = useT()
  const [send, sending] = useSendMessage()
  const [body, setBody] = useState("")
  const [templateId, setTemplateId] = useState<string | null>(null)
  const submit = async () => {
    const text = body.trim()
    if (!text && !templateId) return
    const ok = await send({ input: { conversationId, body: text || null, templateId } })
    if (ok) {
      setBody("")
      setTemplateId(null)
    }
  }
  return (
    <div className="border-t bg-card p-2 pb-[max(0.5rem,env(safe-area-inset-bottom))]">
      {!canReply ? <p className="mb-2 rounded-lg bg-warning/15 px-3 py-2 text-xs text-warning-foreground dark:text-warning">{t.inbox.cannotReply}</p> : null}
      <form
        className="flex items-end gap-1.5"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <QuickReplies
          onPick={(text, id) => {
            setBody(text)
            setTemplateId(id)
          }}
        />
        <Textarea
          value={body}
          onChange={(e) => {
            setBody(e.target.value)
            if (!e.target.value) setTemplateId(null)
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing && window.matchMedia("(min-width: 768px)").matches) {
              e.preventDefault()
              void submit()
            }
          }}
          rows={1}
          placeholder={t.inbox.typeMessage}
          aria-label={t.inbox.typeMessage}
          disabled={!canReply && !templateId}
          className="max-h-36 min-h-10 resize-none"
        />
        <Button type="submit" size="icon" disabled={sending || (!body.trim() && !templateId)} aria-label={t.inbox.send}>
          <SendIcon />
        </Button>
      </form>
    </div>
  )
}

function LabelEditor({ conversation }: { conversation: ConversationListFieldsFragment }) {
  const t = useT()
  const { add, remove } = useConversationLabels()
  const [label, setLabel] = useState("")
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.inbox.labels}>
          <TagIcon />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-64 space-y-2">
        <p className="text-sm font-semibold">{t.inbox.labels}</p>
        <div className="flex flex-wrap gap-1">
          {conversation.labels.map((l) => (
            <Badge key={l} variant="secondary" className="gap-1">
              {l}
              <button type="button" aria-label={`${t.common.delete} ${l}`} onClick={() => void remove[0]({ id: conversation.id, label: l })}>
                <XIcon className="size-3" />
              </button>
            </Badge>
          ))}
        </div>
        <form
          className="flex gap-1"
          onSubmit={async (e) => {
            e.preventDefault()
            if (label.trim() && (await add[0]({ id: conversation.id, label: label.trim() }))) setLabel("")
          }}
        >
          <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder={t.inbox.addLabel} aria-label={t.inbox.addLabel} />
          <Button type="submit" size="sm" disabled={add[1]}>
            {t.common.add}
          </Button>
        </form>
      </PopoverContent>
    </Popover>
  )
}

/** Middle pane: header (status/assign/labels), message bubbles, composer with quick replies. */
export function MessageThread({ conversationId, onBack, headerExtra }: { conversationId: string; onBack?: () => void; headerExtra?: React.ReactNode }) {
  const t = useT()
  const locale = useLocale()
  const { user } = useSession()
  const [{ data, fetching, error }, refetch] = useConversation(conversationId, { first: 50 })
  const [markRead] = useMarkConversationRead()
  const [setStatus] = useSetConversationStatus()
  const [assign] = useAssignConversation()
  const bottomRef = useRef<HTMLDivElement>(null)
  const conv = data?.conversation
  const poll = useCallback(() => refetch({ requestPolicy: "network-only" }), [refetch])
  usePoll(poll, 8000)

  const unread = conv?.unreadCount ?? 0
  useEffect(() => {
    if (unread > 0) void markRead({ id: conversationId })
  }, [conversationId, unread, markRead])

  const messages = [...(conv?.messages ?? [])].sort((a, b) => a.sentAt.localeCompare(b.sentAt))
  const lastId = messages[messages.length - 1]?.id
  useLayoutEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end" })
  }, [lastId])

  if (!conv) {
    if (error) return <ErrorState className="m-4" error={error} onRetry={poll} />
    return fetching ? <ListSkeleton className="p-4" rows={6} /> : <EmptyState className="m-4" title={t.errors.notFound} />
  }

  let lastDay = ""
  return (
    <div className="flex h-full min-h-0 min-w-0 flex-col">
      <header className="flex items-center gap-2 border-b bg-card px-2 py-2 md:px-4">
        {onBack ? (
          <Button variant="ghost" size="icon" onClick={onBack} aria-label={t.common.back} className="md:hidden">
            <ArrowLeftIcon />
          </Button>
        ) : null}
        <PlatformIcon platform={conv.platform} className="size-8" />
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">{conversationTitle(conv)}</p>
          <p className="truncate text-xs text-muted-foreground">
            {t.inbox.via(conv.socialAccount.name)}
            {conv.assignedTo ? ` · ${t.inbox.assignedTo}: ${conv.assignedTo.name}` : ""}
          </p>
        </div>
        <Select value={conv.status} onValueChange={(s) => void setStatus({ id: conv.id, status: s as ConversationStatus })}>
          <SelectTrigger size="sm" className="w-24 shrink-0 md:w-28" aria-label={t.common.status}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CONVERSATION_STATUSES.map((s) => (
              <SelectItem key={s} value={s}>
                {enumLabel(t, "ConversationStatus", s)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={conv.assignedTo?.id === user?.id ? t.inbox.unassign : t.inbox.assignToMe}
          title={conv.assignedTo?.id === user?.id ? t.inbox.unassign : t.inbox.assignToMe}
          onClick={() => void assign({ id: conv.id, userId: conv.assignedTo?.id === user?.id ? null : (user?.id ?? null) })}
          className={conv.assignedTo?.id === user?.id ? "text-primary" : undefined}
        >
          <UserPlusIcon />
        </Button>
        <LabelEditor conversation={conv} />
        {headerExtra}
      </header>
      <div className="min-h-0 flex-1 space-y-2 overflow-y-auto bg-muted/30 px-3 py-4 md:px-6" aria-live="polite">
        {messages.length === 0 ? <p className="py-10 text-center text-sm text-muted-foreground">{t.inbox.noMessages}</p> : null}
        {messages.map((m) => {
          const day = formatDate(m.sentAt, locale)
          const divider = day !== lastDay
          lastDay = day
          return (
            <div key={m.id} className="space-y-2">
              {divider ? <p className="py-1 text-center text-[11px] font-medium text-muted-foreground">{day}</p> : null}
              <MessageBubble m={m} />
            </div>
          )
        })}
        <div ref={bottomRef} />
      </div>
      <Composer conversationId={conv.id} canReply={conv.canReply} />
    </div>
  )
}
