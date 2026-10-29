"use client"

import { useState } from "react"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { PageHeader } from "@/components/shared"
import { CreateSubscriptionSheet, SubscriptionTable } from "@/modules/subscription"
import { OrdersTable } from "@/modules/storefront"
import { useT } from "@/i18n"

export default function SubscriptionsPage() {
  const t = useT()
  const [open, setOpen] = useState(false)
  const newButton = (
    <Button onClick={() => setOpen(true)}>
      <PlusIcon /> {t.subscriptions.new}
    </Button>
  )
  return (
    <>
      <PageHeader title={t.subscriptions.title} description={t.subscriptions.subtitle} actions={newButton} />
      <Tabs defaultValue="subscriptions">
        <TabsList>
          <TabsTrigger value="subscriptions">{t.subscriptions.title}</TabsTrigger>
          <TabsTrigger value="orders">{t.storefront.orders}</TabsTrigger>
        </TabsList>
        <TabsContent value="subscriptions" className="pt-3">
          <SubscriptionTable emptyAction={newButton} />
        </TabsContent>
        <TabsContent value="orders" className="pt-3">
          <OrdersTable />
        </TabsContent>
      </Tabs>
      <CreateSubscriptionSheet open={open} onOpenChange={setOpen} />
    </>
  )
}
