"use client"

import Link from "next/link"
import { useParams } from "next/navigation"
import { ArrowLeftIcon } from "lucide-react"
import { InvoiceDetail } from "@/modules/billing"
import { useT } from "@/i18n"

export default function InvoiceDetailPage() {
  const t = useT()
  const { id } = useParams<{ id: string }>()
  return (
    <div className="space-y-4">
      <Link href="/invoices" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeftIcon className="size-4" /> {t.billing.invoices}
      </Link>
      <InvoiceDetail invoiceId={id} />
    </div>
  )
}
