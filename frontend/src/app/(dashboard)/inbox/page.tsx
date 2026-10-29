"use client"

import { Suspense, useState } from "react"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { InfoIcon, MessagesSquareIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { EmptyState, ListSkeleton } from "@/components/shared"
import type { PickedCustomer } from "@/modules/customer"
import { ConversationCustomerPanel, ConversationList, MessageThread, useConversation } from "@/modules/inbox"
import { CreateSubscriptionSheet } from "@/modules/subscription"
import { cn } from "@/lib/utils"
import { useT } from "@/i18n"

function CustomerPane({ conversationId, onNewSubscription }: { conversationId: string; onNewSubscription: (c: PickedCustomer) => void }) {
  const [{ data }] = useConversation(conversationId, { first: 50 })
  if (!data?.conversation) return <ListSkeleton rows={4} />
  return <ConversationCustomerPanel conversation={data.conversation} onNewSubscription={onNewSubscription} />
}

function Inbox() {
  const t = useT()
  const router = useRouter()
  const pathname = usePathname()
  const params = useSearchParams()
  const selected = params.get("c")
  const [panelOpen, setPanelOpen] = useState(false)
  const [subFor, setSubFor] = useState<PickedCustomer | null>(null)

  const hrefFor = (id: string | null) => {
    const next = new URLSearchParams(params.toString())
    if (id) next.set("c", id)
    else next.delete("c")
    return `${pathname}?${next.toString()}`
  }
  const select = (id: string | null) => router.push(hrefFor(id), { scroll: false })

  return (
    // full-bleed on the shell: cancel main padding, fill viewport minus header (and bottom nav on phones)
    <div className="-mx-4 -mt-4 -mb-[calc(var(--bottom-nav-height)+env(safe-area-inset-bottom)+1rem)] h-[calc(100dvh-var(--topbar-height)-var(--bottom-nav-height)-env(safe-area-inset-bottom))] md:-mx-8 md:-mt-7 md:-mb-12 md:h-[calc(100dvh-4rem)] 2xl:-mx-10">
      <div className="grid h-full grid-cols-1 bg-card md:grid-cols-[20rem_minmax(0,1fr)] xl:grid-cols-[22rem_minmax(0,1fr)_20rem]">
        <section className={cn("min-h-0 min-w-0 border-r", selected && "hidden md:block")} aria-label={t.inbox.title}>
          <ConversationList selectedId={selected} hrefFor={(c) => hrefFor(c.id)} initialSearch={params.get("search") ?? undefined} />
        </section>
        <section className={cn("min-h-0 min-w-0 bg-background", !selected && "hidden md:block")}>
          {selected ? (
            <MessageThread
              conversationId={selected}
              onBack={() => select(null)}
              headerExtra={
                <Button variant="ghost" size="icon-sm" className="xl:hidden" aria-label={t.inbox.customerPanel} onClick={() => setPanelOpen(true)}>
                  <InfoIcon />
                </Button>
              }
            />
          ) : (
            <div className="flex h-full items-center justify-center p-6">
              <EmptyState icon={MessagesSquareIcon} title={t.inbox.selectConversation} className="border-none bg-transparent" />
            </div>
          )}
        </section>
        <aside className="scroll-thin hidden min-h-0 overflow-y-auto border-l p-4 xl:block" aria-label={t.inbox.customerPanel}>
          {selected ? <CustomerPane conversationId={selected} onNewSubscription={setSubFor} /> : null}
        </aside>
      </div>
      <Sheet open={panelOpen} onOpenChange={setPanelOpen}>
        <SheetContent className="w-full overflow-y-auto sm:max-w-sm">
          <SheetHeader>
            <SheetTitle>{t.inbox.customerPanel}</SheetTitle>
          </SheetHeader>
          <div className="px-4 pb-6">
            {selected ? (
              <CustomerPane
                conversationId={selected}
                onNewSubscription={(c) => {
                  setPanelOpen(false)
                  setSubFor(c)
                }}
              />
            ) : null}
          </div>
        </SheetContent>
      </Sheet>
      <CreateSubscriptionSheet open={subFor !== null} onOpenChange={(o) => !o && setSubFor(null)} defaultCustomer={subFor} />
    </div>
  )
}

export default function InboxPage() {
  return (
    <Suspense>
      <Inbox />
    </Suspense>
  )
}
