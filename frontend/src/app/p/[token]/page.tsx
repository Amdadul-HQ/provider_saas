import type { Metadata } from "next"
import { ClientOnly } from "@/components/shared/client-only"
import { PortalMetaDocument } from "@/gql/graphql"
import bn from "@/i18n/bn"
import { serverQuery } from "@/lib/graphql/server"
import { PortalView } from "@/modules/storefront"

type Props = { params: Promise<{ token: string }> }

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { token } = await params
  // the token is a credential: never cache this response in Next's data cache
  const data = await serverQuery(PortalMetaDocument, { token }, { revalidate: false })
  const store = data?.portal?.storefront
  if (!store) return {}
  return { title: { absolute: `${bn.portal.title} · ${store.displayName}` } }
}

export default async function PortalPage({ params }: Props) {
  const { token } = await params
  return (
    <ClientOnly>
      <PortalView token={token} />
    </ClientOnly>
  )
}
