"use client"

import { useParams } from "next/navigation"
import { ClientOnly, ListSkeleton } from "@/components/shared"
import { AcceptInviteView } from "@/modules/auth"

export default function InvitePage() {
  const { token } = useParams<{ token: string }>()
  return (
    <ClientOnly fallback={<ListSkeleton rows={3} />}>
      {/* full reload: drops any previous tenant's urql cache and session state */}
      <AcceptInviteView token={token} onAccepted={() => window.location.assign("/dashboard")} />
    </ClientOnly>
  )
}
