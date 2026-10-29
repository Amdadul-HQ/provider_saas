import type { Metadata, Viewport } from "next"
import { Inter, Noto_Sans_Bengali } from "next/font/google"
import { Providers } from "./providers"
import "./globals.css"

const inter = Inter({ variable: "--font-inter", subsets: ["latin"], display: "swap" })
const bengali = Noto_Sans_Bengali({
  variable: "--font-bengali",
  subsets: ["bengali"],
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
    { media: "(prefers-color-scheme: light)", color: "#1f6f6f" },
    { media: "(prefers-color-scheme: dark)", color: "#14262b" },
  ],
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
}

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="bn" suppressHydrationWarning>
      <body className={`${inter.variable} ${bengali.variable}`}>
        <Providers>{children}</Providers>
      </body>
    </html>
  )
}
