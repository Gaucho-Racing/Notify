import { LogOut, Menu, Settings } from "lucide-react"
import { useEffect, useState } from "react"
import { Link, useLocation, useNavigate } from "react-router-dom"

import { NotifyMark } from "@/components/NotifyMark"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/lib/auth"
import { activeNavLocation, isNavActive, visibleNavGroups } from "@/lib/nav"
import { cn } from "@/lib/utils"

function useUTCClock() {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const interval = window.setInterval(() => setNow(new Date()), 1000)
    return () => window.clearInterval(interval)
  }, [])
  return now.toISOString().slice(11, 19)
}

function initials(name: string) {
  return name
    .split(" ")
    .map((part) => part[0])
    .filter(Boolean)
    .slice(0, 2)
    .join("")
    .toUpperCase()
}

function HeaderUserMenu() {
  const navigate = useNavigate()
  const { user, isLoading, logout } = useAuth()

  if (isLoading || !user) {
    return <Skeleton className="size-8 rounded-full" />
  }

  const name = `${user.first_name} ${user.last_name}`.trim() || user.username || "Notify user"
  const email = user.email || user.username

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button className="rounded-full outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring/35 focus-visible:ring-offset-2">
          <Avatar className="size-8 cursor-pointer">
            <AvatarImage src={user.avatar_url} alt={name} />
            <AvatarFallback>{initials(name)}</AvatarFallback>
          </Avatar>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={10} className="w-56">
        <DropdownMenuLabel className="flex flex-col">
          <span className="text-sm font-medium">{name}</span>
          <span className="text-xs font-normal text-muted-foreground">{email}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => navigate("/settings")}>
          <Settings className="size-4" />
          Settings
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={logout} className="text-destructive focus:text-destructive">
          <LogOut className="size-4" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function AppHeader() {
  const { pathname } = useLocation()
  const location = activeNavLocation(pathname)
  const { isAdmin } = useAuth()
  const clock = useUTCClock()

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-border bg-background/80 px-4 backdrop-blur-md lg:px-8">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="icon" className="lg:hidden">
            <Menu className="size-4" />
            <span className="sr-only">Open navigation</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          {visibleNavGroups(isAdmin).map((group, index) => (
            <DropdownMenuGroup key={group.label}>
              {index > 0 && <DropdownMenuSeparator />}
              <DropdownMenuLabel className="font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">
                {group.label}
              </DropdownMenuLabel>
              {group.items.map((item) => (
                <DropdownMenuItem key={item.to} asChild>
                  <Link to={item.to} className={cn(isNavActive(pathname, item.to) && "text-primary")}>
                    <item.icon className="size-4" />
                    {item.label}
                  </Link>
                </DropdownMenuItem>
              ))}
            </DropdownMenuGroup>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <Link to="/dashboard" className="flex items-center gap-2 lg:hidden">
        <NotifyMark className="size-4 text-primary" />
        <span className="font-heading text-base font-semibold lowercase tracking-wide">notify</span>
      </Link>

      <div className="hidden min-w-0 items-center gap-2 font-mono text-[11px] uppercase tracking-[0.14em] lg:flex">
        <span className="text-muted-foreground">{location?.group.label ?? "Notify"}</span>
        {location && (
          <>
            <span className="text-muted-foreground/50">/</span>
            <span className="text-foreground">{location.item.label}</span>
          </>
        )}
      </div>

      <div className="flex-1" />

      <span className="hidden font-mono text-[11px] tabular-nums tracking-wider text-muted-foreground sm:inline">
        {clock} <span className="text-muted-foreground/60">UTC</span>
      </span>

      <HeaderUserMenu />
    </header>
  )
}
