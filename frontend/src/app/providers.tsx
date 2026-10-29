"use client"

import { useEffect, type ReactNode } from "react"
import { ThemeProvider } from "next-themes"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { GraphQLProvider } from "@/lib/graphql/provider"
import { bootstrapSession } from "@/lib/auth/session"
import { useI18nBootstrap } from "@/i18n"

function Bootstrap() {
  useI18nBootstrap()
  useEffect(() => {
    void bootstrapSession()
    if ("serviceWorker" in navigator && process.env.NODE_ENV === "production") {
      navigator.serviceWorker.register("/sw.js").catch(() => {
        /* PWA is progressive: ignore registration failures */
      })
    }
  }, [])
  return null
}

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="light" enableSystem disableTransitionOnChange>
      <GraphQLProvider>
        <TooltipProvider delayDuration={300}>
          <Bootstrap />
          {children}
          <Toaster position="top-center" richColors closeButton />
        </TooltipProvider>
      </GraphQLProvider>
    </ThemeProvider>
  )
}
