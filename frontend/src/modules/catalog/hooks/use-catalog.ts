"use client"

import { useQuery } from "urql"
import {
  CreateProductDocument,
  DeleteProductDocument,
  DeleteProductPlanDocument,
  ProductDocument,
  ProductsDocument,
  UpdateProductDocument,
  UpsertProductPlanDocument,
  type PageInput,
  type ProductFilter,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useProducts(filter: ProductFilter = {}, page?: PageInput) {
  return useQuery({ query: ProductsDocument, variables: { filter, page }, context: typenameContext("Product", "ProductPlan") })
}

export function useProduct(id: string | null | undefined) {
  return useQuery({ query: ProductDocument, variables: { id: id ?? "" }, pause: !id, context: typenameContext("Product", "ProductPlan") })
}

export function useCreateProduct() {
  const t = useT()
  return useAction(CreateProductDocument, { success: t.catalog.created, typenames: ["Product"] })
}

export function useUpdateProduct() {
  const t = useT()
  return useAction(UpdateProductDocument, { success: t.catalog.updated })
}

export function useDeleteProduct() {
  const t = useT()
  return useAction(DeleteProductDocument, { success: t.catalog.deleted, typenames: ["Product"] })
}

export function useUpsertPlan() {
  const t = useT()
  return useAction(UpsertProductPlanDocument, { success: t.catalog.planSaved, typenames: ["Product", "ProductPlan"] })
}

export function useDeletePlan() {
  const t = useT()
  return useAction(DeleteProductPlanDocument, { success: t.catalog.planDeleted, typenames: ["Product", "ProductPlan"] })
}
