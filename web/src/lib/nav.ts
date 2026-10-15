import { AppWindow, Bot, KeyRound, LayoutDashboard, Mail, MessageSquare, Phone, Send, Settings, type LucideIcon } from "lucide-react"

import type { Channel } from "@/lib/notify"

export type NavItem = {
  to: string
  label: string
  icon: LucideIcon
  adminOnly?: boolean
}

export type NavGroup = {
  label: string
  channel?: Channel
  items: NavItem[]
}

export const NAV_GROUPS: NavGroup[] = [
  {
    label: "Overview",
    items: [
      { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
      { to: "/playground", label: "Playground", icon: Send, adminOnly: true },
    ],
  },
  {
    label: "Email",
    channel: "EMAIL",
    items: [
      { to: "/email-senders", label: "Senders", icon: Mail, adminOnly: true },
      { to: "/resend-accounts", label: "Resend accounts", icon: KeyRound, adminOnly: true },
    ],
  },
  {
    label: "SMS",
    channel: "SMS",
    items: [
      { to: "/sms-senders", label: "Senders", icon: MessageSquare, adminOnly: true },
      { to: "/twilio-accounts", label: "Twilio accounts", icon: Phone, adminOnly: true },
    ],
  },
  {
    label: "Discord",
    channel: "DISCORD",
    items: [{ to: "/discord-bots", label: "Bots", icon: Bot, adminOnly: true }],
  },
  {
    label: "System",
    items: [
      { to: "/applications", label: "Applications", icon: AppWindow, adminOnly: true },
      { to: "/settings", label: "Settings", icon: Settings },
    ],
  },
]

export function visibleNavGroups(isAdmin: boolean) {
  return NAV_GROUPS.map((group) => ({ ...group, items: group.items.filter((item) => isAdmin || !item.adminOnly) })).filter(
    (group) => group.items.length > 0,
  )
}

export function isNavActive(pathname: string, target: string) {
  return pathname === target || pathname.startsWith(`${target}/`)
}

export function activeNavLocation(pathname: string) {
  for (const group of NAV_GROUPS) {
    const item = group.items.find((candidate) => isNavActive(pathname, candidate.to))
    if (item) return { group, item }
  }
  return null
}
