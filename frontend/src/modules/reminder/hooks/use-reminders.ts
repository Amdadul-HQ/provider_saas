"use client"

import { useQuery } from "urql"
import {
  CancelReminderDocument,
  DeleteReminderRuleDocument,
  ReminderRulesDocument,
  RemindersDocument,
  SendReminderNowDocument,
  UpsertReminderRuleDocument,
  type PageInput,
  type ReminderStatus,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useReminderRules() {
  return useQuery({ query: ReminderRulesDocument, context: typenameContext("ReminderRule") })
}

export function useReminders(vars: { status?: ReminderStatus; customerId?: string; page?: PageInput }) {
  return useQuery({ query: RemindersDocument, variables: vars, context: typenameContext("Reminder") })
}

export function useUpsertReminderRule() {
  const t = useT()
  return useAction(UpsertReminderRuleDocument, { success: t.reminders.saved, typenames: ["ReminderRule"] })
}

export function useDeleteReminderRule() {
  const t = useT()
  return useAction(DeleteReminderRuleDocument, { success: t.reminders.deleted, typenames: ["ReminderRule"] })
}

export function useSendReminderNow() {
  const t = useT()
  return useAction(SendReminderNowDocument, { success: t.subscriptions.reminderSent, typenames: ["Reminder"] })
}

export function useCancelReminder() {
  const t = useT()
  return useAction(CancelReminderDocument, { success: t.reminders.cancelled, typenames: ["Reminder"] })
}
