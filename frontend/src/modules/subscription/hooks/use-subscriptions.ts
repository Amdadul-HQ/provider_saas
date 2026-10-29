"use client"

import { useQuery } from "urql"
import {
  CancelSubscriptionDocument,
  CreateSubscriptionDocument,
  ExpiringSubscriptionsDocument,
  ExtendSubscriptionDocument,
  MarkDeliveredDocument,
  ReassignSlotDocument,
  RenewSubscriptionDocument,
  SubscriptionDocument,
  SubscriptionsDocument,
  type PageInput,
  type SubscriptionFilter,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useSubscriptions(filter: SubscriptionFilter, page: PageInput, opts: { pause?: boolean } = {}) {
  return useQuery({ query: SubscriptionsDocument, variables: { filter, page }, pause: opts.pause, context: typenameContext("Subscription") })
}

export function useSubscription(id: string | null | undefined) {
  return useQuery({ query: SubscriptionDocument, variables: { id: id ?? "" }, pause: !id, context: typenameContext("Subscription") })
}

export function useExpiringSubscriptions(days = 7) {
  return useQuery({ query: ExpiringSubscriptionsDocument, variables: { days }, context: typenameContext("Subscription") })
}

const AFFECTED = ["Subscription", "Customer", "Slot", "MasterAccount", "Invoice", "Payment", "Product"]

export function useCreateSubscription() {
  const t = useT()
  return useAction(CreateSubscriptionDocument, { success: t.subscriptions.created, typenames: AFFECTED })
}

export function useRenewSubscription() {
  const t = useT()
  return useAction(RenewSubscriptionDocument, { success: t.subscriptions.renewed, typenames: AFFECTED })
}

export function useMarkDelivered() {
  const t = useT()
  return useAction(MarkDeliveredDocument, { success: t.subscriptions.delivered })
}

export function useCancelSubscription() {
  const t = useT()
  return useAction(CancelSubscriptionDocument, { success: t.subscriptions.cancelled, typenames: AFFECTED })
}

export function useExtendSubscription() {
  const t = useT()
  return useAction(ExtendSubscriptionDocument, { success: t.subscriptions.extended })
}

export function useReassignSlot() {
  const t = useT()
  return useAction(ReassignSlotDocument, { success: t.inventory.slotSaved, typenames: ["Slot", "MasterAccount"] })
}
