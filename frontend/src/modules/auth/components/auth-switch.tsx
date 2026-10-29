"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

/** Pill switch between log in and sign up; the thumb slides to the active side. */
export function AuthSwitch({ className }: { className?: string }) {
  const t = useT()
  const pathname = usePathname()
  const onRegister = pathname.startsWith("/register")
  const tabs = [
    { href: "/login", label: t.auth.login, active: !onRegister },
    { href: "/register", label: t.auth.register, active: onRegister },
  ]
  return (
    <nav aria-label={t.shell.menu} className={cn("relative grid h-11 grid-cols-2 rounded-full bg-muted p-1", className)}>
      <span
        aria-hidden
        className={cn(
          "absolute inset-y-1 left-1 w-[calc(50%-0.25rem)] rounded-full bg-card shadow-sm transition-transform duration-(--dur) ease-(--ease-smooth)",
          onRegister && "translate-x-full",
        )}
      />
      {tabs.map((tab) => (
        <Link
          key={tab.href}
          href={tab.href}
          replace
          aria-current={tab.active ? "page" : undefined}
          className={cn(
            "relative z-10 flex items-center justify-center rounded-full text-sm font-semibold transition-colors duration-(--dur-fast) outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
            tab.active ? "text-foreground" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {tab.label}
        </Link>
      ))}
    </nav>
  )
}
