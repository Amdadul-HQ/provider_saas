"use client"

import { useState } from "react"
import Link from "next/link"
import { useParams, useRouter } from "next/navigation"
import { ArrowLeftIcon, CreditCardIcon, LinkIcon, MessagesSquareIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ConfirmDialog, EmptyState, ErrorState, PageSkeleton, SectionCard } from "@/components/shared"
import { RoleGate } from "@/lib/auth"
import { CustomerFormDialog, CustomerIdentities, CustomerNotes, CustomerProfile, useCustomer, useDeleteCustomer } from "@/modules/customer"
import { CreateSubscriptionSheet, SubscriptionTable } from "@/modules/subscription"
import { InvoiceTable, LedgerTable, RecordPaymentDialog } from "@/modules/billing"
import { ReminderHistory } from "@/modules/reminder"
import { useCreatePortalLink } from "@/modules/storefront"
import { useT } from "@/i18n"

export default function CustomerDetailPage() {
  const t = useT()
  const router = useRouter()
  const { id } = useParams<{ id: string }>()
  const [{ data, fetching, error }, refetch] = useCustomer(id)
  const [editOpen, setEditOpen] = useState(false)
  const [subOpen, setSubOpen] = useState(false)
  const [payOpen, setPayOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [del, deleting] = useDeleteCustomer()
  const [portalLink, creatingLink] = useCreatePortalLink()
  const customer = data?.customer

  const back = (
    <Link href="/customers" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
      <ArrowLeftIcon className="size-4" /> {t.nav.customers}
    </Link>
  )

  if (!customer) {
    return (
      <div className="space-y-4">
        {back}
        {error ? <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} /> : fetching ? <PageSkeleton /> : <EmptyState title={t.customers.notFound} />}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {back}
      <CustomerProfile
        customer={customer}
        actions={
          <>
            <Button onClick={() => setSubOpen(true)}>
              <PlusIcon /> {t.subscriptions.new}
            </Button>
            <Button variant="outline" onClick={() => setPayOpen(true)}>
              <CreditCardIcon /> {t.billing.recordPayment}
            </Button>
            <Button
              variant="outline"
              disabled={creatingLink}
              onClick={async () => {
                const res = await portalLink({ customerId: customer.id })
                if (res?.createPortalLink) {
                  await navigator.clipboard.writeText(res.createPortalLink).catch(() => undefined)
                  toast.success(t.customers.portalLinkCreated)
                }
              }}
            >
              <LinkIcon /> {t.customers.portalLink}
            </Button>
            <Button variant="ghost" size="icon" aria-label={t.common.edit} onClick={() => setEditOpen(true)}>
              <PencilIcon />
            </Button>
            <RoleGate roles={["OWNER", "ADMIN"]}>
              <Button variant="ghost" size="icon" aria-label={t.common.delete} onClick={() => setDeleteOpen(true)}>
                <Trash2Icon />
              </Button>
            </RoleGate>
          </>
        }
      />
      <Tabs defaultValue="subscriptions">
        <div className="-mx-4 overflow-x-auto px-4 md:mx-0 md:px-0">
          <TabsList>
            <TabsTrigger value="subscriptions">{t.customers.tabs.subscriptions}</TabsTrigger>
            <TabsTrigger value="invoices">{t.customers.tabs.invoices}</TabsTrigger>
            <TabsTrigger value="ledger">{t.customers.tabs.ledger}</TabsTrigger>
            <TabsTrigger value="conversations">{t.customers.tabs.conversations}</TabsTrigger>
            <TabsTrigger value="notes">{t.customers.tabs.notes}</TabsTrigger>
            <TabsTrigger value="reminders">{t.customers.tabs.reminders}</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="subscriptions" className="pt-3">
          <SubscriptionTable customerId={customer.id} />
        </TabsContent>
        <TabsContent value="invoices" className="pt-3">
          <InvoiceTable customerId={customer.id} onRowClick={(i) => router.push(`/invoices/${i.id}`)} />
        </TabsContent>
        <TabsContent value="ledger" className="pt-3">
          <LedgerTable customerId={customer.id} />
        </TabsContent>
        <TabsContent value="conversations" className="pt-3">
          {/* ConversationFilter has no customerId yet: show linked identities + jump to inbox search */}
          <SectionCard
            title={t.customers.identities}
            actions={
              <Button variant="outline" size="sm" asChild>
                <Link href={`/inbox?search=${encodeURIComponent(customer.name)}`}>
                  <MessagesSquareIcon /> {t.nav.inbox}
                </Link>
              </Button>
            }
          >
            <CustomerIdentities identities={customer.identities} />
          </SectionCard>
        </TabsContent>
        <TabsContent value="notes" className="pt-3">
          <CustomerNotes customerId={customer.id} />
        </TabsContent>
        <TabsContent value="reminders" className="pt-3">
          <ReminderHistory customerId={customer.id} />
        </TabsContent>
      </Tabs>

      <CustomerFormDialog open={editOpen} onOpenChange={setEditOpen} customer={customer} />
      <CreateSubscriptionSheet open={subOpen} onOpenChange={setSubOpen} defaultCustomer={{ id: customer.id, name: customer.name, phone: customer.phone }} />
      <RecordPaymentDialog open={payOpen} onOpenChange={setPayOpen} customerId={customer.id} defaultAmount={customer.dueBalance} description={customer.name} />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t.common.delete}
        description={t.customers.deleteConfirm}
        destructive
        loading={deleting}
        onConfirm={async () => {
          if (await del({ id: customer.id })) router.replace("/customers")
        }}
      />
    </div>
  )
}
