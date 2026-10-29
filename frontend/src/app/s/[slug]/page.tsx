import type { Metadata } from "next"
import { ClientOnly } from "@/components/shared/client-only"
import { StorefrontMetaDocument } from "@/gql/graphql"
import { serverQuery } from "@/lib/graphql/server"
import { PublicStorefrontView } from "@/modules/storefront"

type Props = { params: Promise<{ slug: string }> }

/** `<title>` is the seller's store name, not SellerDesk (BUG-13); falls back to the root title. */
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params
  const data = await serverQuery(StorefrontMetaDocument, { slug })
  const store = data?.publicStorefront
  if (!store) return {}
  return {
    title: { absolute: store.displayName },
    description: store.tagline ?? undefined,
    openGraph: { title: store.displayName, description: store.tagline ?? undefined },
  }
}

export default async function StorefrontPage({ params }: Props) {
  const { slug } = await params
  // page data is still fetched client-side (urql); only the metadata is fetched on the server
  return (
    <ClientOnly>
      <PublicStorefrontView slug={slug} />
    </ClientOnly>
  )
}
