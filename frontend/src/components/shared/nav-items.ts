import {
  BellRingIcon,
  BoxesIcon,
  CreditCardIcon,
  LayoutDashboardIcon,
  LifeBuoyIcon,
  MegaphoneIcon,
  MessagesSquareIcon,
  PackageIcon,
  ReceiptTextIcon,
  RepeatIcon,
  SettingsIcon,
  Share2Icon,
  UsersIcon,
  type LucideIcon,
} from "lucide-react"
import type { Dict } from "@/i18n"

export type NavKey =
  | "dashboard"
  | "inbox"
  | "customers"
  | "subscriptions"
  | "products"
  | "inventory"
  | "invoices"
  | "payments"
  | "social"
  | "content"
  | "tickets"
  | "reminders"
  | "settings"

export type NavItem = { key: NavKey; href: string; icon: LucideIcon; section: keyof Dict["nav"]["sections"] }

export const NAV_ITEMS: NavItem[] = [
  { key: "dashboard", href: "/dashboard", icon: LayoutDashboardIcon, section: "sell" },
  { key: "inbox", href: "/inbox", icon: MessagesSquareIcon, section: "sell" },
  { key: "customers", href: "/customers", icon: UsersIcon, section: "sell" },
  { key: "subscriptions", href: "/subscriptions", icon: RepeatIcon, section: "sell" },
  { key: "invoices", href: "/invoices", icon: ReceiptTextIcon, section: "money" },
  { key: "payments", href: "/payments", icon: CreditCardIcon, section: "money" },
  { key: "products", href: "/products", icon: PackageIcon, section: "stock" },
  { key: "inventory", href: "/inventory", icon: BoxesIcon, section: "stock" },
  { key: "social", href: "/social", icon: Share2Icon, section: "channels" },
  { key: "content", href: "/content", icon: MegaphoneIcon, section: "channels" },
  { key: "tickets", href: "/tickets", icon: LifeBuoyIcon, section: "support" },
  { key: "reminders", href: "/reminders", icon: BellRingIcon, section: "support" },
  { key: "settings", href: "/settings", icon: SettingsIcon, section: "admin" },
]

/** Bottom tab bar on phones: 4 primary destinations + "More". */
export const MOBILE_PRIMARY: NavKey[] = ["dashboard", "inbox", "customers", "subscriptions"]

export function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`)
}
