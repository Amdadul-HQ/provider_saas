"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { Button } from "@/components/ui/button"
import { BrandMark, LanguageToggle, ThemeToggle } from "@/components/shared"
import { useSession } from "@/lib/auth"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

/**
 * Public site header shared by the landing page and the auth pages, so both use the same
 * `max-w-6xl` container, links and actions.
 * - `overlay` (landing): fixed, transparent over the dark hero, solid once the page scrolls.
 * - `solid`: sticky card surface from the start (for pages without a dark hero under the header).
 * Login/signup use `overlay` too: their layout puts the hero colour under the whole header.
 */
export function SiteHeader({ variant = "overlay" }: { variant?: "overlay" | "solid" }) {
  const t = useT()
  const pathname = usePathname()
  const { status } = useSession()
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    if (variant === "solid") return
    const onScroll = () => setScrolled(window.scrollY > 24)
    onScroll()
    window.addEventListener("scroll", onScroll, { passive: true })
    return () => window.removeEventListener("scroll", onScroll)
  }, [variant])

  const solid = variant === "solid" || scrolled
  const onLanding = pathname === "/"
  const signedIn = status === "authenticated"
  // anchors scroll in place on the landing page and jump back to it from login/signup
  const anchor = (id: string) => (onLanding ? `#${id}` : `/#${id}`)
  const links = [
    { href: anchor("features"), label: t.landing.nav.features },
    { href: anchor("pricing"), label: t.landing.nav.pricing },
    { href: anchor("faq"), label: t.landing.nav.faq },
  ]

  return (
    <header
      className={cn(
        "inset-x-0 top-0 z-50 pt-[env(safe-area-inset-top)] transition-[background-color,box-shadow,color] duration-(--dur)",
        // solid: the bottom hairline is an inset shadow so the header stays exactly 4rem tall
        variant === "solid" ? "sticky shadow-[inset_0_-1px_0_var(--color-border)]" : "fixed",
        solid ? "bg-card/85 text-foreground backdrop-blur-xl" : "bg-transparent text-white",
        variant === "overlay" && scrolled && "shadow-xs",
      )}
    >
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-4 md:px-6">
        <Link href="/" className="flex items-center gap-2.5 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring/60">
          <BrandMark className="size-8 rounded-lg text-base" />
          <span className="font-heading text-lg font-bold tracking-tight">{t.meta.appName}</span>
        </Link>
        <nav className="hidden items-center gap-1 md:flex" aria-label={t.shell.menu}>
          {links.map((l) => (
            <Link
              key={l.href}
              href={l.href}
              className={cn("rounded-lg px-3 py-2 text-sm font-medium transition-colors", solid ? "text-muted-foreground hover:text-foreground" : "text-white/80 hover:text-white")}
            >
              {l.label}
            </Link>
          ))}
        </nav>
        <div className="flex items-center gap-1.5">
          <ThemeToggle className={cn(!solid && "text-white hover:bg-white/10 hover:text-white")} />
          <LanguageToggle className={cn(!solid && "text-white hover:bg-white/10 hover:text-white")} />
          {signedIn ? (
            <Button asChild size="sm" className="h-9 rounded-full px-4">
              <Link href="/dashboard">{t.landing.nav.openDashboard}</Link>
            </Button>
          ) : (
            <>
              <Button
                asChild
                variant="ghost"
                size="sm"
                className={cn("hidden h-9 rounded-full sm:inline-flex", !solid && "text-white hover:bg-white/10 hover:text-white", pathname === "/login" && (solid ? "bg-muted" : "bg-white/15"))}
              >
                <Link href="/login" aria-current={pathname === "/login" ? "page" : undefined}>
                  {t.auth.login}
                </Link>
              </Button>
              <Button asChild size="sm" className={cn("h-9 rounded-full px-4", !solid && "bg-white text-hero-to shadow-md hover:bg-white/90")}>
                <Link href="/register" aria-current={pathname === "/register" ? "page" : undefined}>
                  {t.landing.nav.startFree}
                </Link>
              </Button>
            </>
          )}
        </div>
      </div>
    </header>
  )
}
