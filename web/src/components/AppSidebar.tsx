import { Bell, Bot, KeyRound, LayoutDashboard, Mail, Settings } from "lucide-react"
import { Link, useLocation } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { useAuth } from "@/lib/auth"
import { envLabel, useNotifyStatus } from "@/lib/status"
import { cn } from "@/lib/utils"

const navItems = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/email-senders", label: "Email senders", icon: Mail, adminOnly: true },
  { to: "/resend-accounts", label: "Resend accounts", icon: KeyRound, adminOnly: true },
  { to: "/discord-bots", label: "Discord bots", icon: Bot, adminOnly: true },
  { to: "/settings", label: "Settings", icon: Settings },
]

function isActive(currentPath: string, target: string) {
  return currentPath === target || currentPath.startsWith(`${target}/`)
}

export function AppSidebar() {
  const { pathname } = useLocation()
  const { env } = useNotifyStatus()
  const { isAdmin } = useAuth()
  const environment = envLabel(env)

  return (
    <aside className="hidden border-r border-sidebar-border/70 bg-sidebar text-sidebar-foreground lg:sticky lg:top-0 lg:flex lg:h-svh lg:flex-col">
      <div className="flex h-16 items-center gap-3 border-b border-sidebar-border/70 px-4">
        <div className="flex size-9 items-center justify-center rounded-lg bg-gradient-to-br from-gr-purple to-gr-pink text-primary-foreground shadow-sm shadow-primary/25">
          <Bell className="size-5" />
        </div>
        <div className="min-w-0 text-base font-semibold leading-none">Notify</div>
      </div>
      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
        {navItems.filter((item) => isAdmin || !item.adminOnly).map((item) => {
          const active = isActive(pathname, item.to)
          return (
            <Link
              key={item.to}
              to={item.to}
              className={cn(
                "flex h-10 items-center gap-2 rounded-lg px-3 text-sm font-medium transition-colors",
                active
                  ? "bg-primary text-primary-foreground shadow-sm shadow-primary/15"
                  : "text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
              )}
            >
              <item.icon className="size-4" />
              <span>{item.label}</span>
            </Link>
          )
        })}
      </nav>
      {environment && (
        <div className="px-4 pb-4">
          <Badge variant="secondary" className="w-full justify-center py-1 font-mono text-[11px]">
            {environment}
          </Badge>
        </div>
      )}
    </aside>
  )
}
