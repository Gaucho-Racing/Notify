import type { ReactNode } from "react"

import { Eyebrow } from "@/components/Telemetry"
import { cn } from "@/lib/utils"

export function PageContainer({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <main className={cn("mx-auto w-full max-w-6xl px-4 py-6 lg:px-8 lg:py-10", className)}>
      {children}
    </main>
  )
}

export function PageHeader({
  title,
  eyebrow,
  description,
  action,
}: {
  title: string
  eyebrow?: ReactNode
  description?: string
  action?: ReactNode
}) {
  return (
    <div className="mb-8 flex flex-col gap-4 border-b border-border pb-6 sm:flex-row sm:items-end sm:justify-between">
      <div className="space-y-2">
        {eyebrow && <div>{typeof eyebrow === "string" ? <Eyebrow>{eyebrow}</Eyebrow> : eyebrow}</div>}
        <h1 className="font-heading text-3xl font-semibold tracking-tight">{title}</h1>
        {description && <p className="max-w-2xl text-sm text-muted-foreground">{description}</p>}
      </div>
      {action && <div className="shrink-0 self-start sm:self-auto">{action}</div>}
    </div>
  )
}
