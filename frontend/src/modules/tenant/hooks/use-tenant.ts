"use client"

import { useQuery } from "urql"
import {
  InviteMemberDocument,
  MarkAllNotificationsReadDocument,
  MarkNotificationReadDocument,
  NotificationsDocument,
  RemoveMemberDocument,
  ResendInviteDocument,
  RotateSmsWebhookTokenDocument,
  TenantSettingsDocument,
  UpdateMemberRoleDocument,
  UpdateTenantDocument,
  AuditLogsDocument,
  type PageInput,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useTenantSettings() {
  return useQuery({ query: TenantSettingsDocument, context: typenameContext("Tenant", "TenantMember") })
}

export function useUpdateTenant() {
  const t = useT()
  return useAction(UpdateTenantDocument, { success: t.settings.saved })
}

export function useInviteMember() {
  const t = useT()
  return useAction(InviteMemberDocument, { success: t.settings.invited, typenames: ["TenantMember", "Tenant"] })
}

export function useResendInvite() {
  const t = useT()
  return useAction(ResendInviteDocument, { success: t.settings.inviteResent, typenames: ["TenantMember"] })
}

/** Returns the new SMS-forwarder token; the API never returns it again. */
export function useRotateSmsWebhookToken() {
  const t = useT()
  return useAction(RotateSmsWebhookTokenDocument, { success: t.settings.smsTokenRotated })
}

export function useUpdateMemberRole() {
  const t = useT()
  return useAction(UpdateMemberRoleDocument, { success: t.settings.roleUpdated })
}

export function useRemoveMember() {
  const t = useT()
  return useAction(RemoveMemberDocument, { success: t.settings.removed, typenames: ["TenantMember", "Tenant"] })
}

export function useNotifications(unreadOnly = false, page?: PageInput) {
  return useQuery({ query: NotificationsDocument, variables: { unreadOnly, page }, context: typenameContext("Notification") })
}

export function useMarkNotificationRead() {
  return useAction(MarkNotificationReadDocument, { typenames: ["Notification"], silent: true })
}

export function useMarkAllNotificationsRead() {
  return useAction(MarkAllNotificationsReadDocument, { typenames: ["Notification"] })
}

export function useAuditLogs(page?: PageInput) {
  return useQuery({ query: AuditLogsDocument, variables: { page }, context: typenameContext("AuditLog") })
}
