"use client"

import Link from "next/link"
import { useParams } from "next/navigation"
import { ArrowLeftIcon } from "lucide-react"
import { TicketDetail } from "@/modules/ticket"
import { useT } from "@/i18n"

export default function TicketDetailPage() {
  const t = useT()
  const { id } = useParams<{ id: string }>()
  return (
    <div className="space-y-4">
      <Link href="/tickets" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeftIcon className="size-4" /> {t.tickets.title}
      </Link>
      <TicketDetail ticketId={id} />
    </div>
  )
}
