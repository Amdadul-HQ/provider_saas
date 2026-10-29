"use client"

import { useQuery } from "urql"
import {
  AddLedgerAdjustmentDocument,
  CreateInvoiceDocument,
  IgnorePaymentInboxDocument,
  InvoiceDocument,
  InvoicesDocument,
  LedgerDocument,
  MatchPaymentInboxDocument,
  PaymentInboxDocument,
  PaymentsDocument,
  RecordPaymentDocument,
  RejectPaymentDocument,
  SendInvoiceDocument,
  VerifyPaymentDocument,
  VoidInvoiceDocument,
  type InvoiceFilter,
  type PageInput,
  type PaymentFilter,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

const MONEY_TYPES = ["Invoice", "Payment", "LedgerEntry", "Customer", "PaymentInboxItem"]

export function useInvoices(filter: InvoiceFilter, page: PageInput) {
  return useQuery({ query: InvoicesDocument, variables: { filter, page }, context: typenameContext("Invoice") })
}

export function useInvoice(id: string) {
  return useQuery({ query: InvoiceDocument, variables: { id }, context: typenameContext("Invoice", "Payment") })
}

export function usePayments(filter: PaymentFilter, page: PageInput) {
  return useQuery({ query: PaymentsDocument, variables: { filter, page }, context: typenameContext("Payment") })
}

export function usePaymentInbox(status: string | undefined, page?: PageInput) {
  return useQuery({ query: PaymentInboxDocument, variables: { status, page }, context: typenameContext("PaymentInboxItem") })
}

export function useLedger(customerId: string) {
  return useQuery({ query: LedgerDocument, variables: { customerId }, context: typenameContext("LedgerEntry") })
}

export function useCreateInvoice() {
  const t = useT()
  return useAction(CreateInvoiceDocument, { success: t.billing.invoiceCreated, typenames: MONEY_TYPES })
}

export function useSendInvoice() {
  const t = useT()
  return useAction(SendInvoiceDocument, { success: t.billing.sent })
}

export function useVoidInvoice() {
  const t = useT()
  return useAction(VoidInvoiceDocument, { success: t.billing.voided, typenames: MONEY_TYPES })
}

export function useRecordPayment() {
  const t = useT()
  return useAction(RecordPaymentDocument, { success: t.billing.paymentRecorded, typenames: MONEY_TYPES })
}

export function useVerifyPayment() {
  const t = useT()
  return useAction(VerifyPaymentDocument, { success: t.billing.verified, typenames: MONEY_TYPES })
}

export function useRejectPayment() {
  const t = useT()
  return useAction(RejectPaymentDocument, { success: t.billing.rejected, typenames: MONEY_TYPES })
}

export function useMatchPaymentInbox() {
  const t = useT()
  return useAction(MatchPaymentInboxDocument, { success: t.billing.matched, typenames: MONEY_TYPES })
}

export function useIgnorePaymentInbox() {
  const t = useT()
  return useAction(IgnorePaymentInboxDocument, { success: t.billing.ignored, typenames: ["PaymentInboxItem"] })
}

export function useAddLedgerAdjustment() {
  const t = useT()
  return useAction(AddLedgerAdjustmentDocument, { success: t.billing.adjusted, typenames: ["LedgerEntry", "Customer"] })
}
