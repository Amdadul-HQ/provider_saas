import type { Metadata } from "next"
import bn from "@/i18n/bn"
import { LandingPage } from "@/modules/marketing"

// server-rendered in Bengali (default locale) for search engines and link previews
export const metadata: Metadata = {
  title: { absolute: bn.landing.metaTitle },
  description: bn.landing.metaDescription,
  openGraph: { title: bn.landing.metaTitle, description: bn.landing.metaDescription, type: "website", locale: "bn_BD" },
}

export default function Home() {
  return <LandingPage />
}
