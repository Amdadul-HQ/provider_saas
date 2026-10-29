"use client"

import { useQuery } from "urql"
import {
  AddConversationLabelDocument,
  AssignConversationDocument,
  ConversationDocument,
  ConversationsDocument,
  DeleteMessageTemplateDocument,
  InboxCountsDocument,
  LinkConversationCustomerDocument,
  MarkConversationReadDocument,
  MessageTemplatesDocument,
  RemoveConversationLabelDocument,
  SendMessageDocument,
  SetConversationStatusDocument,
  UpsertMessageTemplateDocument,
  type ConversationFilter,
  type PageInput,
  type TemplateCategory,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useConversations(filter: ConversationFilter, page: PageInput) {
  return useQuery({ query: ConversationsDocument, variables: { filter, page }, context: typenameContext("Conversation") })
}

export function useConversation(id: string | null | undefined, page?: PageInput) {
  return useQuery({
    query: ConversationDocument,
    variables: { id: id ?? "", page },
    pause: !id,
    context: typenameContext("Conversation", "Message"),
  })
}

export function useInboxCounts(pause = false) {
  return useQuery({ query: InboxCountsDocument, pause, context: typenameContext("Conversation") })
}

export function useMessageTemplates(category?: TemplateCategory) {
  return useQuery({ query: MessageTemplatesDocument, variables: { category }, context: typenameContext("MessageTemplate") })
}

export function useSendMessage() {
  return useAction(SendMessageDocument, { typenames: ["Message", "Conversation"] })
}

export function useAssignConversation() {
  return useAction(AssignConversationDocument)
}

export function useSetConversationStatus() {
  const t = useT()
  return useAction(SetConversationStatusDocument, { success: t.inbox.statusChanged })
}

export function useMarkConversationRead() {
  return useAction(MarkConversationReadDocument, { silent: true })
}

export function useConversationLabels() {
  const add = useAction(AddConversationLabelDocument)
  const remove = useAction(RemoveConversationLabelDocument)
  return { add, remove }
}

export function useLinkConversationCustomer() {
  const t = useT()
  return useAction(LinkConversationCustomerDocument, { success: t.inbox.linked })
}

export function useUpsertTemplate() {
  const t = useT()
  return useAction(UpsertMessageTemplateDocument, { success: t.settings.templateSaved, typenames: ["MessageTemplate"] })
}

export function useDeleteTemplate() {
  const t = useT()
  return useAction(DeleteMessageTemplateDocument, { success: t.settings.templateDeleted, typenames: ["MessageTemplate"] })
}
