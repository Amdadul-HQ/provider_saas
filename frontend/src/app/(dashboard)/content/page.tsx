"use client"

import { PageHeader } from "@/components/shared"
import { PostComposer, PostList, PriceListButton } from "@/modules/content"
import { useT } from "@/i18n"

export default function ContentPage() {
  const t = useT()
  return (
    <>
      <PageHeader title={t.content.title} description={t.content.subtitle} actions={<PriceListButton />} />
      <div className="grid gap-4 xl:grid-cols-[28rem_1fr]">
        <div className="xl:sticky xl:top-20 xl:self-start">
          <PostComposer />
        </div>
        <PostList />
      </div>
    </>
  )
}
