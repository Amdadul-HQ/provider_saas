"use client"

import { useState } from "react"
import { PencilIcon, PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { PageHeader } from "@/components/shared"
import type { ProductFieldsFragment } from "@/gql/graphql"
import { RoleGate } from "@/lib/auth"
import { PlansEditor, ProductFormDialog, ProductGrid, useProduct } from "@/modules/catalog"
import { enumLabel, useT } from "@/i18n"

function ProductSheet({ productId, onOpenChange }: { productId: string | null; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [{ data }] = useProduct(productId)
  const [editOpen, setEditOpen] = useState(false)
  const product = data?.product
  return (
    <Sheet open={productId !== null} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            {product?.name}
            <RoleGate roles={["OWNER", "ADMIN"]}>
              <Button variant="ghost" size="icon-sm" aria-label={t.common.edit} onClick={() => setEditOpen(true)}>
                <PencilIcon />
              </Button>
            </RoleGate>
          </SheetTitle>
          <SheetDescription>{product ? `${product.category ?? ""} · ${enumLabel(t, "DeliveryType", product.deliveryType)}` : null}</SheetDescription>
        </SheetHeader>
        {product ? (
          <div className="space-y-4 px-4 pb-6">
            {product.description ? <p className="text-sm text-muted-foreground">{product.description}</p> : null}
            <h3 className="text-sm font-semibold">{t.catalog.plans}</h3>
            <PlansEditor productId={product.id} plans={product.plans} />
          </div>
        ) : null}
        <ProductFormDialog open={editOpen} onOpenChange={setEditOpen} product={product} />
      </SheetContent>
    </Sheet>
  )
}

export default function ProductsPage() {
  const t = useT()
  const [createOpen, setCreateOpen] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)
  const newButton = (
    <RoleGate roles={["OWNER", "ADMIN"]}>
      <Button onClick={() => setCreateOpen(true)}>
        <PlusIcon /> {t.catalog.new}
      </Button>
    </RoleGate>
  )
  return (
    <>
      <PageHeader title={t.catalog.title} description={t.catalog.subtitle} actions={newButton} />
      <ProductGrid onSelect={(p: ProductFieldsFragment) => setSelected(p.id)} emptyAction={newButton} />
      <ProductFormDialog open={createOpen} onOpenChange={setCreateOpen} onSaved={(p) => setSelected(p.id)} />
      <ProductSheet productId={selected} onOpenChange={(o) => !o && setSelected(null)} />
    </>
  )
}
