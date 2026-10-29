"use client"

import { useState } from "react"
import Link from "next/link"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { SectionCard } from "@/components/shared"
import { AtRiskList, ChannelStatsList, DashboardStatsCards, ProductStatsList, RevenueChart, TodayFocus } from "@/modules/analytics"
import { CreateSubscriptionSheet, ExpiringList } from "@/modules/subscription"
import { useT } from "@/i18n"

export default function DashboardPage() {
  const t = useT()
  const [newOpen, setNewOpen] = useState(false)
  return (
    <>
      <div className="space-y-5 md:space-y-6">
        <TodayFocus
          action={
            <Button onClick={() => setNewOpen(true)} size="lg" className="w-full bg-white text-hero-to shadow-md hover:bg-white/90 md:w-auto">
              <PlusIcon /> {t.subscriptions.new}
            </Button>
          }
        />
        <DashboardStatsCards />
        <div className="grid gap-4 lg:grid-cols-[1.6fr_1fr]">
          <SectionCard title={t.dashboard.revenueChart}>
            <RevenueChart />
          </SectionCard>
          <SectionCard
            title={t.dashboard.expiringList}
            actions={
              <Button variant="ghost" size="sm" asChild>
                <Link href="/subscriptions">{t.common.view}</Link>
              </Button>
            }
            bodyClassName="pb-2 md:pb-2"
          >
            <ExpiringList />
          </SectionCard>
        </div>
        <div className="grid gap-4 lg:grid-cols-3">
          <SectionCard title={t.dashboard.atRisk} bodyClassName="pb-2 md:pb-2">
            <AtRiskList />
          </SectionCard>
          <SectionCard title={t.dashboard.productStats} bodyClassName="pb-2 md:pb-2">
            <ProductStatsList />
          </SectionCard>
          <SectionCard title={t.dashboard.channelStats}>
            <ChannelStatsList />
          </SectionCard>
        </div>
      </div>
      <CreateSubscriptionSheet open={newOpen} onOpenChange={setNewOpen} />
    </>
  )
}
