import { cn } from "@/lib/utils"

export function NotifyMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="square" aria-hidden className={cn(className)}>
      <path d="M3 12h4l2.5-7 5 14 2.5-7h4" />
    </svg>
  )
}
