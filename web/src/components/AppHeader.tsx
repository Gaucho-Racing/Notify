import { Bell, Bot, KeyRound, LayoutDashboard, LogOut, Mail, Menu, MessageSquare, Phone, Settings } from "lucide-react"
import { Link, useLocation, useNavigate } from "react-router-dom"

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/lib/auth"
import { cn } from "@/lib/utils"

const mobileItems = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/email-senders", label: "Email senders", icon: Mail, adminOnly: true },
  { to: "/resend-accounts", label: "Resend accounts", icon: KeyRound, adminOnly: true },
  { to: "/sms-senders", label: "SMS senders", icon: MessageSquare, adminOnly: true },
  { to: "/twilio-accounts", label: "Twilio accounts", icon: Phone, adminOnly: true },
  { to: "/discord-bots", label: "Discord bots", icon: Bot, adminOnly: true },
  { to: "/settings", label: "Settings", icon: Settings },
]

function sectionTitle(pathname: string) {
  if (pathname.startsWith("/settings")) return "Settings"
  if (pathname.startsWith("/discord-bots")) return "Discord bots"
  if (pathname.startsWith("/email-senders")) return "Email senders"
  if (pathname.startsWith("/resend-accounts")) return "Resend accounts"
  if (pathname.startsWith("/sms-senders")) return "SMS senders"
  if (pathname.startsWith("/twilio-accounts")) return "Twilio accounts"
  if (pathname.startsWith("/dashboard")) return "Dashboard"
  return "Notify"
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
  const section = sectionTitle(pathname)
  const { isAdmin } = useAuth()

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-border/60 bg-background/90 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/75 lg:px-6">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="icon" className="lg:hidden">
            <Menu className="size-4" />
            <span className="sr-only">Open navigation</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-52">
          <DropdownMenuLabel>Notify</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {mobileItems.filter((item) => isAdmin || !item.adminOnly).map((item) => (
            <DropdownMenuItem key={item.to} asChild>
              <Link
                to={item.to}
                className={cn(pathname.startsWith(item.to) && "text-gr-purple")}
              >
                <item.icon className="size-4" />
                {item.label}
              </Link>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <Link to="/dashboard" className="flex items-center gap-2 lg:hidden">
        <div className="flex size-8 items-center justify-center rounded-lg bg-gradient-to-br from-gr-purple to-gr-pink text-primary-foreground shadow-sm shadow-primary/20">
          <Bell className="size-4" />
        </div>
        <span className="text-sm font-semibold">Notify</span>
      </Link>

      <div className="hidden min-w-0 lg:block">
        <div className="text-lg font-semibold leading-none">{section}</div>
      </div>

      <div className="flex-1" />

      <HeaderUserMenu />
    </header>
  )
}
