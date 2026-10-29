"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { PageHeader } from "@/components/shared"
import { CreateInvoiceDialog, InvoiceTable } from "@/modules/billing"
import { useT } from "@/i18n"

export default function InvoicesPage() {
  const t = useT()
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const newButton = (
    <Button onClick={() => setOpen(true)}>
      <PlusIcon /> {t.billing.newInvoice}
    </Button>
  )
  return (
    <>
      <PageHeader title={t.billing.invoices} description={t.billing.invoicesSubtitle} actions={newButton} />
      <InvoiceTable onRowClick={(i) => router.push(`/invoices/${i.id}`)} emptyAction={newButton} />
      <CreateInvoiceDialog open={open} onOpenChange={setOpen} onCreated={(id) => router.push(`/invoices/${id}`)} />
    </>
  )
}
