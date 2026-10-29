"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { Button } from "@/components/ui/button"
import { BrandMark, LanguageToggle } from "@/components/shared"
import { useSession } from "@/lib/auth"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

/** Sticky header: transparent over the hero, solid card surface once the page scrolls. */
export function LandingHeader() {
  const t = useT()
  const { status } = useSession()
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24)
    onScroll()
    window.addEventListener("scroll", onScroll, { passive: true })
    return () => window.removeEventListener("scroll", onScroll)
  }, [])
  const signedIn = status === "authenticated"
  const links = [
    { href: "#features", label: t.landing.nav.features },
    { href: "#pricing", label: t.landing.nav.pricing },
    { href: "#faq", label: t.landing.nav.faq },
  ]
  return (
    <header
      className={cn(
        "fixed inset-x-0 top-0 z-50 pt-[env(safe-area-inset-top)] transition-[background-color,box-shadow,color] duration-(--dur)",
        scrolled ? "bg-card/85 text-foreground shadow-xs backdrop-blur-xl" : "bg-transparent text-white",
      )}
    >
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-4 md:px-6">
        <Link href="/" className="flex items-center gap-2.5 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring/60">
          <BrandMark className="size-8 rounded-lg text-base" />
          <span className="font-heading text-lg font-bold tracking-tight">{t.meta.appName}</span>
        </Link>
        <nav className="hidden items-center gap-1 md:flex" aria-label={t.shell.menu}>
          {links.map((l) => (
            <a
              key={l.href}
              href={l.href}
              className={cn("rounded-lg px-3 py-2 text-sm font-medium transition-colors", scrolled ? "text-muted-foreground hover:text-foreground" : "text-white/80 hover:text-white")}
            >
              {l.label}
            </a>
          ))}
        </nav>
        <div className="flex items-center gap-1.5">
          <LanguageToggle className={cn(!scrolled && "text-white hover:bg-white/10 hover:text-white")} />
          {signedIn ? (
            <Button asChild size="sm" className="h-9 rounded-full px-4">
              <Link href="/dashboard">{t.landing.nav.openDashboard}</Link>
            </Button>
          ) : (
            <>
              <Button asChild variant="ghost" size="sm" className={cn("hidden h-9 rounded-full sm:inline-flex", !scrolled && "text-white hover:bg-white/10 hover:text-white")}>
                <Link href="/login">{t.auth.login}</Link>
              </Button>
              <Button
                asChild
                size="sm"
                className={cn("h-9 rounded-full px-4", !scrolled && "bg-white text-hero-to shadow-md hover:bg-white/90")}
              >
                <Link href="/register">{t.landing.nav.startFree}</Link>
              </Button>
            </>
          )}
        </div>
      </div>
    </header>
  )
}
