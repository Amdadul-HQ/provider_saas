"use client"

import { useCallback, useState } from "react"
import Link from "next/link"
import { MessagesSquareIcon } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { EmptyState, ErrorState, FilterSelect, ListSkeleton, PlatformIcon, SearchInput } from "@/components/shared"
import type { ConversationFilter, ConversationListFieldsFragment, ConversationStatus, SocialPlatform } from "@/gql/graphql"
import { useCursorPagination } from "@/hooks/use-cursor-pagination"
import { usePoll } from "@/hooks/use-poll"
import { CONVERSATION_STATUSES, SOCIAL_PLATFORMS } from "@/lib/enums"
import { useSession } from "@/lib/auth"
import { enumLabel, useFormat, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useConversations, useInboxCounts } from "../hooks/use-inbox"

type Scope = "all" | "mine" | "unassigned"

export function conversationTitle(c: Pick<ConversationListFieldsFragment, "customer" | "identity" | "externalThreadId">) {
  return c.customer?.name ?? c.identity?.handle ?? c.externalThreadId
}

/** A real link (open in new tab, keyboard, accessible name) rather than a click handler (BUG-10). */
function ConversationRow({ c, selected, href }: { c: ConversationListFieldsFragment; selected: boolean; href: string }) {
  const t = useT()
  const f = useFormat()
  const title = conversationTitle(c)
  const unread = c.unreadCount > 0 ? ` · ${t.inbox.counts.unread} ${f.number(c.unreadCount)}` : ""
  return (
    <Link
      href={href}
      scroll={false}
      aria-current={selected ? "page" : undefined}
      aria-label={`${title}${unread} · ${c.lastMessagePreview ?? ""}`}
      className={cn(
        "mx-2 my-0.5 flex items-start gap-3 rounded-xl px-2.5 py-2.5 text-left transition-[background-color,transform] duration-(--dur-fast) ease-(--ease-smooth) outline-none focus-visible:ring-2 focus-visible:ring-ring/60",
        selected ? "bg-primary/8 ring-1 ring-primary/15" : "hover:bg-muted/70 active:scale-[0.99] active:bg-muted",
      )}
    >
      <div className="relative shrink-0">
        <Avatar className="size-11">
          {c.identity?.avatarUrl ? <AvatarImage src={c.identity.avatarUrl} alt="" /> : null}
          <AvatarFallback className={cn("text-xs font-semibold", c.unreadCount > 0 ? "bg-primary/12 text-primary" : "bg-secondary text-secondary-foreground")}>
            {title.slice(0, 2)}
          </AvatarFallback>
        </Avatar>
        <PlatformIcon platform={c.platform} className="absolute -right-0.5 -bottom-0.5 size-5 ring-2 ring-card [&_svg]:size-2.5" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <p className={cn("truncate text-sm", c.unreadCount > 0 ? "font-semibold" : "font-medium")}>{title}</p>
          <span className={cn("shrink-0 text-[11px]", c.unreadCount > 0 ? "font-medium text-primary" : "text-muted-foreground")}>
            {c.lastMessageAt ? f.relative(c.lastMessageAt) : ""}
          </span>
        </div>
        <div className="flex items-center justify-between gap-2">
          <p className={cn("truncate text-xs", c.unreadCount > 0 ? "text-foreground" : "text-muted-foreground")}>{c.lastMessagePreview ?? "—"}</p>
          {c.unreadCount > 0 ? (
            <span className="flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-primary px-1.5 text-[11px] font-semibold text-primary-foreground">
              {f.number(c.unreadCount)}
            </span>
          ) : null}
        </div>
        {c.labels.length || c.assignedTo ? (
          <div className="mt-1 flex flex-wrap gap-1">
            {c.labels.slice(0, 3).map((l) => (
              <span key={l} className="rounded-md bg-accent px-1.5 text-[10px] font-medium text-accent-foreground">
                {l}
              </span>
            ))}
            {c.assignedTo ? <span className="rounded-md bg-muted px-1.5 text-[10px] text-muted-foreground">@{c.assignedTo.name}</span> : null}
          </div>
        ) : null}
      </div>
    </Link>
  )
}

/** Left pane of the inbox: scope tabs (with counts), search, filters, cursor "load more". */
export function ConversationList({
  selectedId,
  hrefFor,
  initialSearch,
}: {
  selectedId: string | null
  /** URL that opens a conversation (keeps the page's other search params). */
  hrefFor: (c: ConversationListFieldsFragment) => string
  initialSearch?: string
}) {
  const t = useT()
  const f = useFormat()
  const { user } = useSession()
  const [scope, setScope] = useState<Scope>("all")
  const [filter, setFilter] = useState<ConversationFilter>({ status: initialSearch ? undefined : "OPEN", search: initialSearch || undefined })
  const pager = useCursorPagination(30)
  const effective: ConversationFilter = {
    ...filter,
    assignedTo: scope === "mine" ? user?.id : undefined,
    unassigned: scope === "unassigned" ? true : undefined,
  }
  const [{ data, fetching, error }, refetch] = useConversations(effective, pager.page)
  const [{ data: counts }, refetchCounts] = useInboxCounts()
  const poll = useCallback(() => {
    refetch({ requestPolicy: "network-only" })
    refetchCounts({ requestPolicy: "network-only" })
  }, [refetch, refetchCounts])
  usePoll(poll, 15000)
  const conn = data?.conversations
  const update = (patch: Partial<ConversationFilter>) => {
    pager.reset()
    setFilter((f) => ({ ...f, ...patch }))
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="space-y-2 border-b border-border/70 p-3">
        <Tabs
          value={scope}
          onValueChange={(v) => {
            pager.reset()
            setScope(v as Scope)
          }}
        >
          <TabsList className="h-9 w-full rounded-full p-1">
            {(
              [
                ["all", t.inbox.counts.open, counts?.inboxCounts.open],
                ["mine", t.inbox.counts.mine, counts?.inboxCounts.mine],
                ["unassigned", t.inbox.counts.unassigned, counts?.inboxCounts.unassigned],
              ] as const
            ).map(([value, label, count]) => (
              <TabsTrigger key={value} value={value} className="flex-1 rounded-full data-active:shadow-xs">
                {label}
                {typeof count === "number" ? (
                  <span className={cn("tabular rounded-full px-1.5 text-[11px] leading-4", scope === value ? "bg-primary/10 text-primary" : "text-muted-foreground")}>
                    {f.number(count)}
                  </span>
                ) : null}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <SearchInput className="md:max-w-none" value={filter.search ?? ""} onChange={(search) => update({ search: search || undefined })} placeholder={t.inbox.searchPlaceholder} />
        <div className="grid grid-cols-2 gap-2">
          <FilterSelect<SocialPlatform>
            className="md:w-full"
            label={t.reminders.channel}
            value={filter.platform}
            onChange={(platform) => update({ platform })}
            options={SOCIAL_PLATFORMS.map((p) => ({ value: p, label: enumLabel(t, "Platform", p) }))}
          />
          <FilterSelect<ConversationStatus>
            className="md:w-full"
            label={t.common.status}
            value={filter.status}
            onChange={(status) => update({ status })}
            options={CONVERSATION_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "ConversationStatus", s) }))}
          />
        </div>
      </div>
      <div className="scroll-thin min-h-0 flex-1 overflow-y-auto py-1.5">
        {!conn ? (
          error ? (
            <ErrorState compact className="m-3" error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
          ) : fetching ? (
            <ListSkeleton className="p-3" rows={8} />
          ) : null
        ) : conn.edges.length === 0 ? (
          <EmptyState compact className="m-3" icon={MessagesSquareIcon} title={t.inbox.empty} description={t.inbox.emptyHint} />
        ) : (
          <>
            {conn.edges.map(({ node }) => (
              <ConversationRow key={node.id} c={node} selected={node.id === selectedId} href={hrefFor(node)} />
            ))}
            {pager.canPrev || conn.pageInfo.hasNextPage ? (
              <div className="flex justify-between gap-2 p-3">
                <Button variant="ghost" size="sm" disabled={!pager.canPrev} onClick={pager.prev}>
                  {t.common.previous}
                </Button>
                <Button variant="ghost" size="sm" disabled={!conn.pageInfo.hasNextPage} onClick={() => pager.next(conn.pageInfo.endCursor)}>
                  {t.common.next}
                </Button>
              </div>
            ) : null}
          </>
        )}
      </div>
    </div>
  )
}
