import type { ReactNode } from "react"

import type { Channel } from "@/lib/notify"
import { cn } from "@/lib/utils"

const CHANNEL_STYLES: Record<Channel, { label: string; dot: string; text: string; tile: string }> = {
  EMAIL: {
    label: "Email",
    dot: "bg-channel-email",
    text: "text-channel-email",
    tile: "border-channel-email/25 bg-channel-email/10 text-channel-email",
  },
  SMS: {
    label: "SMS",
    dot: "bg-channel-sms",
    text: "text-channel-sms",
    tile: "border-channel-sms/25 bg-channel-sms/10 text-channel-sms",
  },
  DISCORD: {
    label: "Discord",
    dot: "bg-channel-discord",
    text: "text-channel-discord",
    tile: "border-channel-discord/25 bg-channel-discord/10 text-channel-discord",
  },
}

export function ChannelTag({ channel, className }: { channel: Channel; className?: string }) {
  const style = CHANNEL_STYLES[channel]
  return (
    <span className={cn("inline-flex items-center gap-1.5 font-mono text-[10px] font-medium uppercase tracking-[0.14em]", style.text, className)}>
      <span className={cn("size-1.5 rounded-[1px]", style.dot)} />
      {style.label}
    </span>
  )
}

export function ChannelTile({ channel, children }: { channel: Channel; children: ReactNode }) {
  return (
    <div className={cn("flex size-10 shrink-0 items-center justify-center rounded-md border", CHANNEL_STYLES[channel].tile)}>
      {children}
    </div>
  )
}

export function Eyebrow({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={cn("font-mono text-[10px] font-medium uppercase tracking-[0.16em] text-muted-foreground", className)}>
      {children}
    </span>
  )
}

const SIGNAL_STYLES = {
  live: "bg-status-sent shadow-[0_0_8px_var(--status-sent)]",
  idle: "bg-status-skipped",
  warn: "bg-status-pending shadow-[0_0_8px_var(--status-pending)]",
  down: "bg-status-failed shadow-[0_0_8px_var(--status-failed)]",
}

export function SignalDot({ state, pulse = false, className }: { state: keyof typeof SIGNAL_STYLES; pulse?: boolean; className?: string }) {
  return (
    <span
      className={cn(
        "inline-block size-1.5 shrink-0 rounded-full",
        SIGNAL_STYLES[state],
        pulse && "animate-[signal-pulse_2s_ease-in-out_infinite]",
        className,
      )}
    />
  )
}
