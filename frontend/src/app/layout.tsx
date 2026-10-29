import type { Metadata, Viewport } from "next"
import { headers } from "next/headers"
import { Hind_Siliguri, Manrope } from "next/font/google"
import { Providers } from "./providers"
import "./globals.css"

// ADR-009: Manrope carries Latin + digits, Hind Siliguri carries Bengali (the face Bangladeshi
// fintech UIs use); Manrope has no Bengali glyphs so Bengali runs fall through to Hind Siliguri.
const manrope = Manrope({ variable: "--font-manrope", subsets: ["latin"], display: "swap" })
const bengali = Hind_Siliguri({
  variable: "--font-bengali",
  subsets: ["bengali", "latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
})

export const metadata: Metadata = {
  title: { default: "সেলারডেস্ক — SellerDesk", template: "%s · সেলারডেস্ক" },
  description: "ডিজিটাল সাবস্ক্রিপশন বিক্রেতাদের জন্য ইনবক্স, স্লট, ইনভয়েস ও রিমাইন্ডার — এক জায়গায়।",
  manifest: "/manifest.json",
  applicationName: "SellerDesk",
  appleWebApp: { capable: true, title: "সেলারডেস্ক", statusBarStyle: "default" },
  icons: { icon: "/icons/icon.svg", apple: "/icons/icon.svg" },
}

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#1d6b6b" },
    { media: "(prefers-color-scheme: dark)", color: "#14262b" },
  ],
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
}

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  // per-request CSP nonce from middleware (also makes every route dynamic, which nonces require)
  const nonce = (await headers()).get("x-nonce") ?? undefined
  return (
    <html lang="bn" suppressHydrationWarning>
      <body className={`${manrope.variable} ${bengali.variable}`}>
        <Providers nonce={nonce}>{children}</Providers>
      </body>
    </html>
  )
}
