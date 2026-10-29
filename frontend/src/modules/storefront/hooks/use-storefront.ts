"use client"

import { useQuery } from "urql"
import {
  CancelOrderDocument,
  ConfirmOrderDocument,
  CreatePortalLinkDocument,
  OrdersDocument,
  PlaceOrderDocument,
  PortalDocument,
  PortalRequestRenewalDocument,
  PublicOrderDocument,
  PublicStorefrontDocument,
  StorefrontDocument,
  UpsertStorefrontDocument,
  type OrderFilter,
  type PageInput,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useStorefrontSettings() {
  return useQuery({ query: StorefrontDocument, context: typenameContext("Storefront") })
}

export function useUpsertStorefront() {
  const t = useT()
  return useAction(UpsertStorefrontDocument, { success: t.storefront.saved, typenames: ["Storefront"] })
}

export function useOrders(filter: OrderFilter, page?: PageInput) {
  return useQuery({ query: OrdersDocument, variables: { filter, page }, context: typenameContext("Order") })
}

export function useConfirmOrder() {
  const t = useT()
  return useAction(ConfirmOrderDocument, { success: t.storefront.orderConfirmed, typenames: ["Order", "Subscription", "Slot"] })
}

export function useCancelOrder() {
  const t = useT()
  return useAction(CancelOrderDocument, { success: t.storefront.orderCancelled })
}

export function useCreatePortalLink() {
  return useAction(CreatePortalLinkDocument)
}

/* ---- public (no session) ---- */

export function usePublicStorefront(slug: string) {
  return useQuery({ query: PublicStorefrontDocument, variables: { slug }, requestPolicy: "cache-first" })
}

export function usePublicOrder(vars: { slug: string; number: number; phone: string } | null) {
  return useQuery({
    query: PublicOrderDocument,
    variables: vars ?? { slug: "", number: 0, phone: "" },
    pause: !vars,
    requestPolicy: "network-only",
  })
}

export function usePlaceOrder() {
  return useAction(PlaceOrderDocument, { silent: false })
}

export function usePortal(token: string) {
  return useQuery({ query: PortalDocument, variables: { token }, requestPolicy: "network-only" })
}

export function usePortalRenewal() {
  const t = useT()
  return useAction(PortalRequestRenewalDocument, { success: t.portal.renewRequested })
}
