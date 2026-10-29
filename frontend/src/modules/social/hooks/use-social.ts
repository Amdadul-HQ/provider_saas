"use client"

import { useQuery } from "urql"
import {
  CompleteMetaConnectDocument,
  ConnectTelegramDocument,
  ConnectWhatsAppDocument,
  DisconnectSocialAccountDocument,
  SocialAccountsDocument,
  StartMetaConnectDocument,
  SyncSocialAccountDocument,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useSocialAccounts() {
  return useQuery({ query: SocialAccountsDocument, context: typenameContext("SocialAccount") })
}

export function useStartMetaConnect() {
  return useAction(StartMetaConnectDocument)
}

export function useCompleteMetaConnect() {
  return useAction(CompleteMetaConnectDocument, { typenames: ["SocialAccount"] })
}

export function useConnectTelegram() {
  const t = useT()
  return useAction(ConnectTelegramDocument, { success: t.social.completed(1), typenames: ["SocialAccount"] })
}

export function useConnectWhatsApp() {
  const t = useT()
  return useAction(ConnectWhatsAppDocument, { success: t.social.completed(1), typenames: ["SocialAccount"] })
}

export function useDisconnectSocialAccount() {
  const t = useT()
  return useAction(DisconnectSocialAccountDocument, { success: t.social.disconnected, typenames: ["SocialAccount"] })
}

export function useSyncSocialAccount() {
  const t = useT()
  return useAction(SyncSocialAccountDocument, { success: t.social.synced })
}
