import type { Metadata } from "next"
import type { ReactNode } from "react"

// the token in the path is a credential: never index it or leak it through the Referer header
export const metadata: Metadata = { robots: { index: false, follow: false }, referrer: "no-referrer" }

export default function InviteLayout({ children }: { children: ReactNode }) {
  return children
}
