"use client"

import { useState } from "react"
import Link from "next/link"
import { PlusIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { PageHeader, SectionCard } from "@/components/shared"
import { AtRiskList, ChannelStatsList, DashboardStatsCards, ProductStatsList, RevenueChart } from "@/modules/analytics"
import { CreateSubscriptionSheet, ExpiringList } from "@/modules/subscription"
import { useSession } from "@/lib/auth"
import { useT } from "@/i18n"

export default function DashboardPage() {
  const t = useT()
  const { user } = useSession()
  const [newOpen, setNewOpen] = useState(false)
  return (
    <>
      <PageHeader
        title={t.dashboard.title}
        description={user ? `${t.dashboard.subtitle} · ${user.name}` : t.dashboard.subtitle}
        actions={
          <Button onClick={() => setNewOpen(true)}>
            <PlusIcon /> {t.subscriptions.new}
          </Button>
        }
      />
      <div className="space-y-4">
        <DashboardStatsCards />
        <div className="grid gap-4 lg:grid-cols-[1.6fr_1fr]">
          <SectionCard title={t.dashboard.revenueChart}>
            <RevenueChart />
          </SectionCard>
          <SectionCard
            title={t.dashboard.expiringList}
            actions={
              <Button variant="link" size="sm" asChild>
                <Link href="/subscriptions">{t.common.view}</Link>
              </Button>
            }
            bodyClassName="py-1"
          >
            <ExpiringList />
          </SectionCard>
        </div>
        <div className="grid gap-4 lg:grid-cols-3">
          <SectionCard title={t.dashboard.atRisk} bodyClassName="py-1">
            <AtRiskList />
          </SectionCard>
          <SectionCard title={t.dashboard.productStats} bodyClassName="py-1">
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
