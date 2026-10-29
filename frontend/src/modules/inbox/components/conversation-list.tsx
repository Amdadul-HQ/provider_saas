"use client"

import { useCallback, useState } from "react"
import { MessagesSquareIcon } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { EmptyState, ErrorState, FilterSelect, ListSkeleton, PlatformIcon, SearchInput } from "@/components/shared"
import type { ConversationFilter, ConversationListFieldsFragment, ConversationStatus, SocialPlatform } from "@/gql/graphql"
import { useCursorPagination } from "@/hooks/use-cursor-pagination"
import { usePoll } from "@/hooks/use-poll"
import { formatRelative } from "@/lib/dates"
import { CONVERSATION_STATUSES, SOCIAL_PLATFORMS } from "@/lib/enums"
import { useSession } from "@/lib/auth"
import { enumLabel, useLocale, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useConversations, useInboxCounts } from "../hooks/use-inbox"

type Scope = "all" | "mine" | "unassigned"

export function conversationTitle(c: Pick<ConversationListFieldsFragment, "customer" | "identity" | "externalThreadId">) {
  return c.customer?.name ?? c.identity?.handle ?? c.externalThreadId
}

function ConversationRow({ c, selected, onSelect }: { c: ConversationListFieldsFragment; selected: boolean; onSelect: () => void }) {
  const locale = useLocale()
  const title = conversationTitle(c)
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-current={selected ? "true" : undefined}
      className={cn(
        "flex w-full items-start gap-3 border-b px-3 py-3 text-left transition-colors last:border-b-0",
        selected ? "bg-primary/8" : "hover:bg-muted/60 active:bg-muted",
      )}
    >
      <div className="relative shrink-0">
        <Avatar className="size-10">
          {c.identity?.avatarUrl ? <AvatarImage src={c.identity.avatarUrl} alt="" /> : null}
          <AvatarFallback className="text-xs">{title.slice(0, 2)}</AvatarFallback>
        </Avatar>
        <PlatformIcon platform={c.platform} className="absolute -right-1 -bottom-1 size-4.5 ring-2 ring-card [&_svg]:size-2.5" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <p className={cn("truncate text-sm", c.unreadCount > 0 ? "font-semibold" : "font-medium")}>{title}</p>
          <span className="shrink-0 text-[11px] text-muted-foreground">{c.lastMessageAt ? formatRelative(c.lastMessageAt, locale) : ""}</span>
        </div>
        <div className="flex items-center justify-between gap-2">
          <p className={cn("truncate text-xs", c.unreadCount > 0 ? "text-foreground" : "text-muted-foreground")}>{c.lastMessagePreview ?? "—"}</p>
          {c.unreadCount > 0 ? (
            <span className="flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-primary px-1.5 text-[11px] font-semibold text-primary-foreground">
              {c.unreadCount}
            </span>
          ) : null}
        </div>
        {c.labels.length || c.assignedTo ? (
          <div className="mt-1 flex flex-wrap gap-1">
            {c.labels.slice(0, 3).map((l) => (
              <span key={l} className="rounded bg-accent px-1.5 text-[10px] font-medium text-accent-foreground">
                {l}
              </span>
            ))}
            {c.assignedTo ? <span className="rounded bg-muted px-1.5 text-[10px] text-muted-foreground">@{c.assignedTo.name}</span> : null}
          </div>
        ) : null}
      </div>
    </button>
  )
}

/** Left pane of the inbox: scope tabs (with counts), search, filters, cursor "load more". */
export function ConversationList({
  selectedId,
  onSelect,
  initialSearch,
}: {
  selectedId: string | null
  onSelect: (c: ConversationListFieldsFragment) => void
  initialSearch?: string
}) {
  const t = useT()
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
      <div className="space-y-2 border-b p-3">
        <Tabs
          value={scope}
          onValueChange={(v) => {
            pager.reset()
            setScope(v as Scope)
          }}
        >
          <TabsList className="w-full">
            <TabsTrigger value="all" className="flex-1">
              {t.inbox.counts.open} {counts ? <span className="tabular text-muted-foreground">{counts.inboxCounts.open}</span> : null}
            </TabsTrigger>
            <TabsTrigger value="mine" className="flex-1">
              {t.inbox.counts.mine} {counts ? <span className="tabular text-muted-foreground">{counts.inboxCounts.mine}</span> : null}
            </TabsTrigger>
            <TabsTrigger value="unassigned" className="flex-1">
              {t.inbox.counts.unassigned} {counts ? <span className="tabular text-muted-foreground">{counts.inboxCounts.unassigned}</span> : null}
            </TabsTrigger>
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
      <div className="min-h-0 flex-1 overflow-y-auto">
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
              <ConversationRow key={node.id} c={node} selected={node.id === selectedId} onSelect={() => onSelect(node)} />
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
