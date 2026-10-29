"use client"

import { useQuery } from "urql"
import {
  CreateMasterAccountDocument,
  CreateSupplierDocument,
  DeleteSupplierDocument,
  FreeSlotsDocument,
  MasterAccountDocument,
  MasterAccountsDocument,
  RevealMasterAccountCredentialDocument,
  SetMasterAccountCredentialDocument,
  SuppliersDocument,
  UpdateMasterAccountDocument,
  UpdateSlotDocument,
  UpdateSupplierDocument,
  type MasterAccountFilter,
  type PageInput,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useSuppliers() {
  return useQuery({ query: SuppliersDocument, context: typenameContext("Supplier") })
}

export function useSaveSupplier() {
  const t = useT()
  const create = useAction(CreateSupplierDocument, { success: t.inventory.supplierSaved, typenames: ["Supplier"] })
  const update = useAction(UpdateSupplierDocument, { success: t.inventory.supplierSaved })
  return { create, update }
}

export function useDeleteSupplier() {
  const t = useT()
  return useAction(DeleteSupplierDocument, { success: t.inventory.supplierDeleted, typenames: ["Supplier"] })
}

export function useMasterAccounts(filter: MasterAccountFilter, page?: PageInput) {
  return useQuery({ query: MasterAccountsDocument, variables: { filter, page }, context: typenameContext("MasterAccount") })
}

export function useMasterAccount(id: string | null | undefined) {
  return useQuery({ query: MasterAccountDocument, variables: { id: id ?? "" }, pause: !id, context: typenameContext("MasterAccount", "Slot") })
}

export function useFreeSlots(productId: string | null | undefined) {
  return useQuery({ query: FreeSlotsDocument, variables: { productId: productId ?? "" }, pause: !productId, context: typenameContext("Slot") })
}

export function useCreateMasterAccount() {
  const t = useT()
  return useAction(CreateMasterAccountDocument, { success: t.inventory.created, typenames: ["MasterAccount", "Slot", "Product"] })
}

export function useUpdateMasterAccount() {
  const t = useT()
  return useAction(UpdateMasterAccountDocument, { success: t.inventory.updated, typenames: ["Slot"] })
}

export function useSetCredential() {
  const t = useT()
  return useAction(SetMasterAccountCredentialDocument, { success: t.inventory.credentialSaved, typenames: ["MasterAccount"] })
}

/** Reveal is audit-logged server-side; never cache the result (network-only mutation). */
export function useRevealCredential() {
  return useAction(RevealMasterAccountCredentialDocument)
}

export function useUpdateSlot() {
  const t = useT()
  return useAction(UpdateSlotDocument, { success: t.inventory.slotSaved, typenames: ["MasterAccount"] })
}
