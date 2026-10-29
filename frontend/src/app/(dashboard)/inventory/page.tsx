"use client"

import { useState } from "react"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { EmptyState, PageHeader } from "@/components/shared"
import { useIsDesktop } from "@/hooks/use-media-query"
import { RoleGate } from "@/lib/auth"
import { MasterAccountFormDialog, MasterAccountSlots, MasterAccountTable, SupplierPanel } from "@/modules/inventory"
import { useT } from "@/i18n"

export default function InventoryPage() {
  const t = useT()
  const isDesktop = useIsDesktop()
  const [selected, setSelected] = useState<string | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const newButton = (
    <RoleGate roles={["OWNER", "ADMIN"]}>
      <Button onClick={() => setCreateOpen(true)}>
        <PlusIcon /> {t.inventory.newAccount}
      </Button>
    </RoleGate>
  )
  return (
    <>
      <PageHeader title={t.inventory.title} description={t.inventory.subtitle} actions={newButton} />
      <Tabs defaultValue="accounts">
        <TabsList>
          <TabsTrigger value="accounts">{t.inventory.tabs.accounts}</TabsTrigger>
          <TabsTrigger value="suppliers">{t.inventory.tabs.suppliers}</TabsTrigger>
        </TabsList>
        <TabsContent value="accounts" className="pt-3">
          <div className="grid gap-4 xl:grid-cols-[1.3fr_1fr]">
            <MasterAccountTable selectedId={selected} onSelect={(a) => setSelected(a.id)} emptyAction={newButton} />
            {isDesktop ? (
              <div className="xl:sticky xl:top-20 xl:self-start">
                {selected ? <MasterAccountSlots accountId={selected} /> : <EmptyState compact title={t.inventory.selectAccount} />}
              </div>
            ) : null}
          </div>
        </TabsContent>
        <TabsContent value="suppliers" className="pt-3">
          <SupplierPanel />
        </TabsContent>
      </Tabs>
      {!isDesktop ? (
        <Sheet open={selected !== null} onOpenChange={(o) => !o && setSelected(null)}>
          <SheetContent side="bottom" className="max-h-[92dvh] overflow-y-auto rounded-t-2xl">
            <SheetHeader>
              <SheetTitle>{t.inventory.slots}</SheetTitle>
            </SheetHeader>
            <div className="px-4 pb-6">{selected ? <MasterAccountSlots accountId={selected} /> : null}</div>
          </SheetContent>
        </Sheet>
      ) : null}
      <MasterAccountFormDialog open={createOpen} onOpenChange={setCreateOpen} onSaved={(a) => setSelected(a.id)} />
    </>
  )
}
