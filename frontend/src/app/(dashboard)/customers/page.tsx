"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { PageHeader } from "@/components/shared"
import { CustomerFormDialog, CustomerTable } from "@/modules/customer"
import { useT } from "@/i18n"

export default function CustomersPage() {
  const t = useT()
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const newButton = (
    <Button onClick={() => setOpen(true)}>
      <PlusIcon /> {t.customers.new}
    </Button>
  )
  return (
    <>
      <PageHeader title={t.customers.title} description={t.customers.subtitle} actions={newButton} />
      <CustomerTable onRowClick={(c) => router.push(`/customers/${c.id}`)} emptyAction={newButton} />
      <CustomerFormDialog open={open} onOpenChange={setOpen} onSaved={(c) => router.push(`/customers/${c.id}`)} />
    </>
  )
}
