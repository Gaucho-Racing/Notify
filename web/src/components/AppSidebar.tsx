import { Link, useLocation } from "react-router-dom"

import { NotifyMark } from "@/components/NotifyMark"
import { Eyebrow } from "@/components/Telemetry"
import { useAuth } from "@/lib/auth"
import { isNavActive, visibleNavGroups } from "@/lib/nav"
import { cn } from "@/lib/utils"

const CHANNEL_DOTS = {
  EMAIL: "bg-channel-email",
  SMS: "bg-channel-sms",
  DISCORD: "bg-channel-discord",
}

export function AppSidebar() {
  const { pathname } = useLocation()
  const { isAdmin } = useAuth()

  return (
    <aside className="hidden border-r border-sidebar-border bg-sidebar/95 text-sidebar-foreground lg:sticky lg:top-0 lg:flex lg:h-svh lg:flex-col">
      <Link to="/dashboard" className="flex h-14 items-center gap-2.5 border-b border-sidebar-border px-5">
        <NotifyMark className="size-5 text-primary" />
        <span className="font-heading text-lg font-semibold lowercase tracking-wide">notify</span>
      </Link>

      <nav className="flex-1 space-y-5 overflow-y-auto px-3 py-5">
        {visibleNavGroups(isAdmin).map((group) => (
          <div key={group.label}>
            <div className="mb-1.5 flex items-center gap-2 px-2">
              {group.channel && <span className={cn("size-1.5 rounded-[1px]", CHANNEL_DOTS[group.channel])} />}
              <Eyebrow>{group.label}</Eyebrow>
            </div>
            <div className="space-y-px">
              {group.items.map((item) => {
                const active = isNavActive(pathname, item.to)
                return (
                  <Link
                    key={item.to}
                    to={item.to}
                    className={cn(
                      "relative flex h-8 items-center gap-2.5 rounded-sm px-2 text-[13px] transition-colors",
                      active
                        ? "bg-sidebar-accent text-sidebar-accent-foreground before:absolute before:inset-y-1.5 before:-left-3 before:w-0.5 before:bg-primary"
                        : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                    )}
                  >
                    <item.icon className={cn("size-3.5", active && "text-primary")} />
                    <span>{item.label}</span>
                  </Link>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

    </aside>
  )
}
