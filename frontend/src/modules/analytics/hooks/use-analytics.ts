"use client"

import { useQuery } from "urql"
import { AtRiskCustomersDocument, ChannelStatsDocument, DashboardStatsDocument, ProductStatsDocument, RevenueSeriesDocument } from "@/gql/graphql"
import { typenameContext } from "@/lib/graphql/use-action"

const CTX = typenameContext("Subscription", "Payment", "Invoice", "Customer")

export function useDashboardStats() {
  return useQuery({ query: DashboardStatsDocument, context: CTX })
}

export function useRevenueSeries(days = 30) {
  return useQuery({ query: RevenueSeriesDocument, variables: { days }, context: CTX })
}

export function useProductStats(days = 30) {
  return useQuery({ query: ProductStatsDocument, variables: { days }, context: CTX })
}

export function useChannelStats(days = 30) {
  return useQuery({ query: ChannelStatsDocument, variables: { days }, context: CTX })
}

export function useAtRiskCustomers(limit = 10) {
  return useQuery({ query: AtRiskCustomersDocument, variables: { limit }, context: CTX })
}
