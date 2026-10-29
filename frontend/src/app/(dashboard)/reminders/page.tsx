"use client"

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { PageHeader } from "@/components/shared"
import { ReminderHistory, ReminderRules } from "@/modules/reminder"
import { useT } from "@/i18n"

export default function RemindersPage() {
  const t = useT()
  return (
    <>
      <PageHeader title={t.reminders.title} description={t.reminders.subtitle} />
      <Tabs defaultValue="rules">
        <TabsList>
          <TabsTrigger value="rules">{t.reminders.tabs.rules}</TabsTrigger>
          <TabsTrigger value="history">{t.reminders.tabs.history}</TabsTrigger>
        </TabsList>
        <TabsContent value="rules" className="pt-3">
          <ReminderRules />
        </TabsContent>
        <TabsContent value="history" className="pt-3">
          <ReminderHistory />
        </TabsContent>
      </Tabs>
    </>
  )
}
