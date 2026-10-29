"use client"

import { useState } from "react"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { CustomerPicker, CustomerSidePanel, type PickedCustomer } from "@/modules/customer"
import type { ConversationListFieldsFragment } from "@/gql/graphql"
import { useT } from "@/i18n"
import { useLinkConversationCustomer } from "../hooks/use-inbox"

/** Right pane: linked customer's context, or a picker to link one. */
export function ConversationCustomerPanel({
  conversation,
  onNewSubscription,
}: {
  conversation: ConversationListFieldsFragment
  onNewSubscription?: (customer: PickedCustomer) => void
}) {
  const t = useT()
  const [link, linking] = useLinkConversationCustomer()
  const [picked, setPicked] = useState<PickedCustomer | null>(null)
  if (conversation.customer) {
    const c = conversation.customer
    return (
      <CustomerSidePanel
        customerId={c.id}
        actions={
          onNewSubscription ? (
            <Button onClick={() => onNewSubscription({ id: c.id, name: c.name, phone: c.phone })}>
              <PlusIcon /> {t.inbox.newSubscription}
            </Button>
          ) : null
        }
      />
    )
  }
  return (
    <div className="space-y-3 text-sm">
      <p className="text-muted-foreground">{t.inbox.noCustomer}</p>
      <CustomerPicker value={picked} onChange={setPicked} />
      <Button className="w-full" disabled={!picked || linking} onClick={() => picked && void link({ id: conversation.id, customerId: picked.id })}>
        {t.inbox.linkCustomer}
      </Button>
    </div>
  )
}
