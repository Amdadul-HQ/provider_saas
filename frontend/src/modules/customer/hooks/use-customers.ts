"use client"

import { useQuery } from "urql"
import {
  AddCustomerNoteDocument,
  CreateCustomerDocument,
  CustomerDocument,
  CustomerNotesDocument,
  CustomersDocument,
  DeleteCustomerDocument,
  MergeCustomersDocument,
  UpdateCustomerDocument,
  type CustomerFilter,
  type PageInput,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useCustomers(filter: CustomerFilter, page: PageInput, pause = false) {
  return useQuery({ query: CustomersDocument, variables: { filter, page }, context: typenameContext("Customer"), pause })
}

export function useCustomer(id: string) {
  return useQuery({ query: CustomerDocument, variables: { id }, pause: !id, context: typenameContext("Customer", "Subscription") })
}

export function useCustomerNotes(customerId: string) {
  return useQuery({ query: CustomerNotesDocument, variables: { customerId }, context: typenameContext("CustomerNote") })
}

export function useCreateCustomer() {
  const t = useT()
  return useAction(CreateCustomerDocument, { success: t.customers.created, typenames: ["Customer"] })
}

export function useUpdateCustomer() {
  const t = useT()
  return useAction(UpdateCustomerDocument, { success: t.customers.updated })
}

export function useDeleteCustomer() {
  const t = useT()
  return useAction(DeleteCustomerDocument, { success: t.customers.deleted, typenames: ["Customer"] })
}

export function useAddCustomerNote() {
  return useAction(AddCustomerNoteDocument, { typenames: ["CustomerNote"] })
}

export function useMergeCustomers() {
  const t = useT()
  return useAction(MergeCustomersDocument, { success: t.customers.updated, typenames: ["Customer"] })
}
