"use client"

import { CheckIcon, LogOutIcon, Building2Icon, ChevronsUpDownIcon } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { enumLabel, useT } from "@/i18n"
import { useSession } from "@/lib/auth"
import { cn } from "@/lib/utils"
import { useLogout, useSwitchTenant } from "../hooks/use-auth"

function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((p) => p[0] ?? "")
    .join("")
    .toUpperCase()
}

/** Avatar menu: current business, switch business, log out. `variant="sidebar"` for the dark rail. */
export function UserMenu({ variant = "topbar" }: { variant?: "topbar" | "sidebar" }) {
  const t = useT()
  const { user, tenant, role } = useSession()
  const logout = useLogout()
  const { switchTenant } = useSwitchTenant()
  if (!user) return null
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={cn(
          "flex min-h-10 items-center gap-2 rounded-lg px-1.5 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
          variant === "sidebar" ? "w-full hover:bg-sidebar-accent" : "hover:bg-muted",
        )}
        aria-label={t.shell.profile}
      >
        <Avatar className="size-8">
          {user.avatarUrl ? <AvatarImage src={user.avatarUrl} alt="" /> : null}
          <AvatarFallback className="bg-primary/15 text-xs font-semibold text-primary">{initials(user.name)}</AvatarFallback>
        </Avatar>
        {variant === "sidebar" ? (
          <>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{tenant?.name ?? user.name}</span>
              <span className="block truncate text-xs opacity-70">{user.name}</span>
            </span>
            <ChevronsUpDownIcon className="size-4 opacity-60" />
          </>
        ) : null}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel className="space-y-0.5">
          <p className="truncate text-sm font-medium">{user.name}</p>
          <p className="truncate text-xs font-normal text-muted-foreground">
            {user.email ?? user.phone} · {enumLabel(t, "Role", role)}
          </p>
        </DropdownMenuLabel>
        {user.memberships.length > 1 ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="text-xs text-muted-foreground">{t.shell.switchTenant}</DropdownMenuLabel>
            {user.memberships.map((m) => (
              <DropdownMenuItem key={m.id} onSelect={() => m.tenant.id !== tenant?.id && void switchTenant(m.tenant.id)}>
                <Building2Icon />
                <span className="flex-1 truncate">{m.tenant.name}</span>
                {m.tenant.id === tenant?.id ? <CheckIcon className="text-primary" /> : null}
              </DropdownMenuItem>
            ))}
          </>
        ) : null}
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => void logout()}>
          <LogOutIcon /> {t.shell.logout}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
