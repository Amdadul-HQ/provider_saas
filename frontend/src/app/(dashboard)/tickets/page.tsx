"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { PageHeader } from "@/components/shared"
import { CreateTicketDialog, TicketStatsCards, TicketTable } from "@/modules/ticket"
import { useT } from "@/i18n"

export default function TicketsPage() {
  const t = useT()
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const newButton = (
    <Button onClick={() => setOpen(true)}>
      <PlusIcon /> {t.tickets.new}
    </Button>
  )
  return (
    <>
      <PageHeader title={t.tickets.title} description={t.tickets.subtitle} actions={newButton} />
      <div className="space-y-4">
        <TicketStatsCards />
        <TicketTable rowHref={(tk) => `/tickets/${tk.id}`} emptyAction={newButton} />
      </div>
      <CreateTicketDialog open={open} onOpenChange={setOpen} onCreated={(id) => router.push(`/tickets/${id}`)} />
    </>
  )
}
