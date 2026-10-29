"use client"

import { useQuery } from "urql"
import {
  AddTicketCommentDocument,
  CreateTicketDocument,
  ResolveReplacementDocument,
  TicketDocument,
  TicketStatsDocument,
  TicketsDocument,
  UpdateTicketDocument,
  type PageInput,
  type TicketFilter,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useTickets(filter: TicketFilter, page?: PageInput) {
  return useQuery({ query: TicketsDocument, variables: { filter, page }, context: typenameContext("Ticket") })
}

export function useTicket(id: string) {
  return useQuery({ query: TicketDocument, variables: { id }, context: typenameContext("Ticket", "TicketComment") })
}

export function useTicketStats() {
  return useQuery({ query: TicketStatsDocument, context: typenameContext("Ticket") })
}

export function useCreateTicket() {
  const t = useT()
  return useAction(CreateTicketDocument, { success: t.tickets.created, typenames: ["Ticket"] })
}

export function useUpdateTicket() {
  const t = useT()
  return useAction(UpdateTicketDocument, { success: t.tickets.updated, typenames: ["Ticket"] })
}

export function useAddTicketComment() {
  return useAction(AddTicketCommentDocument, { typenames: ["TicketComment", "Ticket"] })
}

export function useResolveReplacement() {
  const t = useT()
  return useAction(ResolveReplacementDocument, { success: t.tickets.resolved, typenames: ["Ticket", "Slot", "Subscription", "MasterAccount"] })
}
