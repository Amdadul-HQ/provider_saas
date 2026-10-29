import type { Metadata } from "next"
import type { ReactNode } from "react"

// magic-link pages must never be indexed or leak the token via referrers
export const metadata: Metadata = { robots: { index: false, follow: false }, referrer: "no-referrer" }

export default function PortalLayout({ children }: { children: ReactNode }) {
  return children
}
