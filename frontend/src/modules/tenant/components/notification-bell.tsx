"use client"

import Link from "next/link"
import { BellIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { ScrollArea } from "@/components/ui/scroll-area"
import { DateText, EmptyState } from "@/components/shared"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useMarkAllNotificationsRead, useMarkNotificationRead, useNotifications } from "../hooks/use-tenant"

export function NotificationBell() {
  const t = useT()
  const [{ data }] = useNotifications(false, { first: 20 })
  const [markRead] = useMarkNotificationRead()
  const [markAll, markingAll] = useMarkAllNotificationsRead()
  const items = data?.notifications ?? []
  const unread = items.filter((n) => !n.readAt).length

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="relative" aria-label={t.shell.notifications}>
          <BellIcon />
          {unread > 0 ? (
            <span className="absolute top-1.5 right-1.5 flex size-4 items-center justify-center rounded-full bg-warning text-[10px] font-bold text-warning-foreground">
              {unread > 9 ? "9+" : unread}
            </span>
          ) : null}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[min(22rem,calc(100vw-1rem))] p-0">
        <div className="flex items-center justify-between border-b px-3 py-2">
          <p className="text-sm font-semibold">{t.shell.notifications}</p>
          {unread > 0 ? (
            <Button variant="ghost" size="sm" disabled={markingAll} onClick={() => void markAll({})}>
              {t.shell.markAllRead}
            </Button>
          ) : null}
        </div>
        {items.length === 0 ? (
          <EmptyState compact title={t.shell.noNotifications} className="m-3 border-none" />
        ) : (
          <ScrollArea className="max-h-96">
            <ul className="divide-y">
              {items.map((n) => {
                const body = (
                  <div className={cn("space-y-0.5 px-3 py-2.5", !n.readAt && "bg-primary/5")}>
                    <p className="text-sm font-medium">{n.title}</p>
                    {n.body ? <p className="line-clamp-2 text-xs text-muted-foreground">{n.body}</p> : null}
                    <DateText value={n.createdAt} relative className="text-[11px] text-muted-foreground" />
                  </div>
                )
                return (
                  <li key={n.id}>
                    {n.link ? (
                      <Link href={n.link} onClick={() => !n.readAt && void markRead({ id: n.id })} className="block hover:bg-muted">
                        {body}
                      </Link>
                    ) : (
                      <button type="button" className="block w-full text-left hover:bg-muted" onClick={() => !n.readAt && void markRead({ id: n.id })}>
                        {body}
                      </button>
                    )}
                  </li>
                )
              })}
            </ul>
          </ScrollArea>
        )}
      </PopoverContent>
    </Popover>
  )
}
