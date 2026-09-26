import { Bot, UserRound } from "lucide-react"

import { ApplicationIcon } from "@/components/ApplicationDisplay"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Skeleton } from "@/components/ui/skeleton"
import type { IdentitySummary } from "@/lib/directory"
import { cn } from "@/lib/utils"

type DisplaySize = "sm" | "md"

function initials(name: string) {
  return name
    .split(/\s+/)
    .map((part) => part[0])
    .filter(Boolean)
    .slice(0, 2)
    .join("")
    .toUpperCase()
}

export function IdentityDisplay({
  entityID,
  identity,
  loading = false,
  size = "md",
  showDetails = true,
  className,
}: {
  entityID: string
  identity?: IdentitySummary
  loading?: boolean
  size?: DisplaySize
  showDetails?: boolean
  className?: string
}) {
  if (!entityID) {
    return (
      <div className={cn("flex min-w-0 items-center gap-2.5", className)}>
        <Avatar size={size === "sm" ? "sm" : "default"}>
          <AvatarFallback><UserRound className="size-3.5" /></AvatarFallback>
        </Avatar>
        <span className={size === "sm" ? "text-xs" : "text-sm"}>Anonymous</span>
      </div>
    )
  }

  if (loading && !identity) {
    return (
      <div className={cn("flex min-w-0 items-center gap-2.5", className)}>
        <Skeleton className={cn("rounded-full", size === "sm" ? "size-6" : "size-8")} />
        <Skeleton className={cn("h-3.5", size === "sm" ? "w-20" : "w-28")} />
      </div>
    )
  }

  const name = identity?.name || entityID
  const isServiceAccount = identity?.type === "SERVICE_ACCOUNT"
  const application = identity?.application

  return (
    <div className={cn("flex min-w-0 items-center gap-2.5", className)}>
      {isServiceAccount && application ? (
        <ApplicationIcon application={application} size={size} />
      ) : (
        <Avatar size={size === "sm" ? "sm" : "default"}>
          {identity?.avatar_url && <AvatarImage src={identity.avatar_url} alt={name} />}
          <AvatarFallback className={isServiceAccount ? "bg-primary text-primary-foreground" : undefined}>
            {isServiceAccount ? <Bot className="size-3.5" /> : initials(name) || "?"}
          </AvatarFallback>
        </Avatar>
      )}
      <span className="flex min-w-0 flex-col leading-tight">
        <span className={cn("truncate font-medium", size === "sm" ? "text-xs" : "text-sm")}>
          {name}
        </span>
        {showDetails && (
          <span className="truncate text-[11px] text-muted-foreground">
            {identity?.username
              ? `@${identity.username}`
              : isServiceAccount
                ? application?.name || "Service account"
                : identity
                  ? "User"
                  : entityID}
          </span>
        )}
      </span>
    </div>
  )
}
