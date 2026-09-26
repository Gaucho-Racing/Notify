import { useState } from "react"

import type { SentinelApplication } from "@/lib/notify"
import { cn } from "@/lib/utils"

type ApplicationVisual = Pick<SentinelApplication, "name" | "client_id" | "icon_url">
type DisplaySize = "sm" | "md"

const iconSizes: Record<DisplaySize, string> = {
  sm: "size-6 rounded-md text-xs",
  md: "size-8 rounded-lg text-sm",
}

export function ApplicationIcon({
  application,
  fallbackName,
  size = "md",
}: {
  application?: ApplicationVisual
  fallbackName?: string
  size?: DisplaySize
}) {
  const [failedURL, setFailedURL] = useState<string | null>(null)
  const name = application?.name || fallbackName || application?.client_id || "Unknown application"
  const imageURL =
    application?.icon_url && failedURL !== application.icon_url ? application.icon_url : null

  return (
    <span
      className={cn(
        iconSizes[size],
        "flex shrink-0 items-center justify-center overflow-hidden",
        !imageURL && "bg-gradient-to-br from-gr-pink to-gr-purple font-semibold text-white",
      )}
    >
      {imageURL ? (
        <img
          src={imageURL}
          alt={name}
          className="size-full object-contain"
          onError={() => setFailedURL(imageURL)}
        />
      ) : (
        (name.slice(0, 1) || "?").toUpperCase()
      )}
    </span>
  )
}

export function ApplicationDisplay({
  clientID,
  application,
  size = "md",
  showClientID = true,
  className,
}: {
  clientID: string
  application?: ApplicationVisual
  size?: DisplaySize
  showClientID?: boolean
  className?: string
}) {
  const name = application?.name || clientID || "Unknown application"
  return (
    <div className={cn("flex min-w-0 items-center gap-2.5", className)}>
      <ApplicationIcon application={application} fallbackName={clientID} size={size} />
      <span className="flex min-w-0 flex-col leading-tight">
        <span className={cn("truncate font-medium", size === "sm" ? "text-xs" : "text-sm")}>
          {name}
        </span>
        {showClientID && clientID && (
          <span className="truncate font-mono text-[11px] text-muted-foreground">{clientID}</span>
        )}
      </span>
    </div>
  )
}
