"use client"

import { useState } from "react"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { PageHeader } from "@/components/shared"
import { PaymentInbox, PaymentTable, RecordPaymentDialog } from "@/modules/billing"
import { useT } from "@/i18n"

export default function PaymentsPage() {
  const t = useT()
  const [open, setOpen] = useState(false)
  return (
    <>
      <PageHeader
        title={t.billing.payments}
        description={t.billing.paymentsSubtitle}
        actions={
          <Button onClick={() => setOpen(true)}>
            <PlusIcon /> {t.billing.recordPayment}
          </Button>
        }
      />
      <Tabs defaultValue="pending">
        <TabsList>
          <TabsTrigger value="pending">{t.billing.tabs.pending}</TabsTrigger>
          <TabsTrigger value="inbox">{t.billing.tabs.inbox}</TabsTrigger>
          <TabsTrigger value="all">{t.billing.tabs.all}</TabsTrigger>
        </TabsList>
        <TabsContent value="pending" className="pt-3">
          <PaymentTable initialStatus="PENDING" />
        </TabsContent>
        <TabsContent value="inbox" className="pt-3">
          <PaymentInbox />
        </TabsContent>
        <TabsContent value="all" className="pt-3">
          <PaymentTable />
        </TabsContent>
      </Tabs>
      <RecordPaymentDialog open={open} onOpenChange={setOpen} />
    </>
  )
}
