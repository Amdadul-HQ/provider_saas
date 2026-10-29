"use client"

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { PageHeader, SectionCard } from "@/components/shared"
import { ChangePasswordForm } from "@/modules/auth"
import { TemplateManager } from "@/modules/inbox"
import { StorefrontEditor } from "@/modules/storefront"
import { MembersPanel, PreferencesPanel, TenantSettingsForm } from "@/modules/tenant"
import { useT } from "@/i18n"

export default function SettingsPage() {
  const t = useT()
  return (
    <>
      <PageHeader title={t.settings.title} description={t.settings.subtitle} />
      <Tabs defaultValue="business">
        <div className="-mx-4 overflow-x-auto px-4 md:mx-0 md:px-0">
          <TabsList>
            <TabsTrigger value="business">{t.settings.tabs.business}</TabsTrigger>
            <TabsTrigger value="members">{t.settings.tabs.members}</TabsTrigger>
            <TabsTrigger value="storefront">{t.settings.tabs.storefront}</TabsTrigger>
            <TabsTrigger value="templates">{t.settings.tabs.templates}</TabsTrigger>
            <TabsTrigger value="language">{t.settings.tabs.language}</TabsTrigger>
            <TabsTrigger value="account">{t.settings.tabs.account}</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="business" className="pt-3">
          <TenantSettingsForm />
        </TabsContent>
        <TabsContent value="members" className="pt-3">
          <MembersPanel />
        </TabsContent>
        <TabsContent value="storefront" className="pt-3">
          <StorefrontEditor />
        </TabsContent>
        <TabsContent value="templates" className="pt-3">
          <TemplateManager />
        </TabsContent>
        <TabsContent value="language" className="pt-3">
          <PreferencesPanel />
        </TabsContent>
        <TabsContent value="account" className="pt-3">
          <SectionCard title={t.auth.changePassword} className="max-w-xl">
            <ChangePasswordForm />
          </SectionCard>
        </TabsContent>
      </Tabs>
    </>
  )
}
