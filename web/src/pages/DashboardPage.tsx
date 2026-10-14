import { useQuery } from "@tanstack/react-query"
import { useState } from "react"
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipContentProps } from "recharts"

import { PageContainer, PageHeader } from "@/components/PageContainer"
import { ChannelTag, Eyebrow, SignalDot } from "@/components/Telemetry"
import { Skeleton } from "@/components/ui/skeleton"
import { apiErrorMessage } from "@/lib/api"
import { getActivityStats, getStats, type Channel, type StatusCounts } from "@/lib/notify"
import { cn } from "@/lib/utils"

const WINDOWS = [7, 30, 90] as const
const CHANNELS: Channel[] = ["EMAIL", "SMS", "DISCORD"]
const CHANNEL_BARS: Record<Channel, string> = {
  EMAIL: "bg-channel-email",
  SMS: "bg-channel-sms",
  DISCORD: "bg-channel-discord",
}

const SERIES = [
  { key: "sent", label: "Sent", color: "var(--status-sent)" },
  { key: "failed", label: "Failed", color: "var(--status-failed)" },
  { key: "pending", label: "Pending", color: "var(--status-pending)" },
  { key: "skipped", label: "Skipped", color: "var(--status-skipped)" },
] as const

function deliveryRate(counts?: StatusCounts) {
  if (!counts) return null
  const attempted = counts.sent + counts.failed
  return attempted === 0 ? null : counts.sent / attempted
}

function formatRate(rate: number | null) {
  return rate === null ? "—" : `${(rate * 100).toFixed(rate === 1 ? 0 : 1)}%`
}

export default function DashboardPage() {
  const [days, setDays] = useState<(typeof WINDOWS)[number]>(30)
  const stats = useQuery({ queryKey: ["stats", days], queryFn: () => getStats(days) })
  const activity = useQuery({ queryKey: ["stats", "activity", days], queryFn: () => getActivityStats(days) })
  const deliveries = stats.data?.deliveries
  const rate = deliveryRate(deliveries)

  const readouts = [
    { label: "Messages", value: stats.data?.messages },
    { label: "Sent", value: deliveries?.sent, signal: "live" as const },
    { label: "Failed", value: deliveries?.failed, signal: "down" as const },
    { label: "Pending", value: deliveries?.pending, signal: "warn" as const },
    { label: "Skipped", value: deliveries?.skipped, signal: "idle" as const },
  ]

  return (
    <PageContainer>
      <PageHeader
        eyebrow={`Telemetry · last ${days} days`}
        title="Dashboard"
        action={
          <div className="inline-flex rounded-md border border-border bg-card/60 p-0.5 font-mono text-[11px]">
            {WINDOWS.map((window) => (
              <button
                key={window}
                type="button"
                onClick={() => setDays(window)}
                className={cn(
                  "rounded-sm px-2.5 py-1 tabular-nums transition-colors",
                  days === window ? "bg-brand text-white" : "text-muted-foreground hover:text-foreground",
                )}
              >
                {window}D
              </button>
            ))}
          </div>
        }
      />

      {stats.error && (
        <div className="mb-4 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {apiErrorMessage(stats.error, "Failed to load stats.")}
        </div>
      )}

      <section className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-3 lg:grid-cols-6">
        <div className="col-span-2 bg-card p-5 sm:col-span-1">
          <Eyebrow>Delivery rate</Eyebrow>
          <div className="mt-2 font-mono text-3xl font-medium tabular-nums text-brand">
            {stats.isLoading ? <Skeleton className="h-9 w-20" /> : formatRate(rate)}
          </div>
        </div>
        {readouts.map((readout, index) => (
          <div key={readout.label} className={cn("bg-card p-5", index === 0 && "col-span-2 sm:col-span-1")}>
            <div className="flex items-center gap-2">
              {readout.signal && <SignalDot state={readout.signal} />}
              <Eyebrow>{readout.label}</Eyebrow>
            </div>
            <div className="mt-2 font-mono text-2xl tabular-nums">
              {stats.isLoading ? <Skeleton className="h-8 w-14" /> : (readout.value ?? 0).toLocaleString()}
            </div>
          </div>
        ))}
      </section>

      <section className="mt-6 rounded-lg border border-border bg-card/90 p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <Eyebrow>Deliveries per day</Eyebrow>
          <div className="flex flex-wrap items-center gap-4">
            {SERIES.map((series) => (
              <span key={series.key} className="flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                <span className="size-2 rounded-[1px]" style={{ background: series.color }} />
                {series.label}
              </span>
            ))}
          </div>
        </div>
        <div className="h-56">
          {activity.isLoading ? (
            <Skeleton className="h-full w-full" />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={activity.data ?? []} margin={{ top: 4, right: 0, bottom: 0, left: -24 }} barCategoryGap="18%">
                <XAxis
                  dataKey="date"
                  tickFormatter={(value: string) => value.slice(5)}
                  tick={{ fill: "var(--muted-foreground)", fontSize: 10, fontFamily: "var(--font-mono)" }}
                  axisLine={{ stroke: "var(--border)" }}
                  tickLine={false}
                  minTickGap={16}
                />
                <YAxis
                  allowDecimals={false}
                  tick={{ fill: "var(--muted-foreground)", fontSize: 10, fontFamily: "var(--font-mono)" }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip cursor={{ fill: "rgb(132 18 252 / 10%)" }} content={(props) => <ActivityTooltip {...props} />} />
                {SERIES.map((series) => (
                  <Bar key={series.key} dataKey={series.key} name={series.label} stackId="deliveries" fill={series.color} />
                ))}
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </section>

      <section className="mt-6 grid gap-4 md:grid-cols-3">
        {CHANNELS.map((channel) => {
          const counts = stats.data?.by_channel[channel]
          const channelRate = deliveryRate(counts)
          return (
            <div key={channel} className="rounded-lg border border-border bg-card/90 p-5">
              <div className="flex items-center justify-between">
                <ChannelTag channel={channel} />
                <span className="font-mono text-xs tabular-nums text-muted-foreground">{formatRate(channelRate)}</span>
              </div>
              <div className="mt-3 font-mono text-2xl tabular-nums">{(counts?.sent ?? 0).toLocaleString()}</div>
              <div className="mt-0.5 font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                sent · {(counts?.failed ?? 0).toLocaleString()} failed
              </div>
              <div className="mt-4 h-1 overflow-hidden rounded-full bg-muted">
                <div className={cn("h-full", CHANNEL_BARS[channel])} style={{ width: `${(channelRate ?? 0) * 100}%` }} />
              </div>
            </div>
          )
        })}
      </section>

      {!!stats.data?.top_errors.length && (
        <section className="mt-6 overflow-hidden rounded-lg border border-border bg-card/90">
          <div className="border-b border-border px-5 py-3">
            <Eyebrow>Top errors</Eyebrow>
          </div>
          <div className="divide-y divide-border">
            {stats.data.top_errors.map((row) => (
              <div key={`${row.channel}-${row.error}`} className="grid grid-cols-[72px_minmax(0,1fr)_auto] items-start gap-4 px-5 py-2.5">
                <ChannelTag channel={row.channel} className="pt-0.5" />
                <span className="break-words font-mono text-xs text-foreground/85">{row.error}</span>
                <span className="font-mono text-xs tabular-nums text-status-failed">{row.count.toLocaleString()}</span>
              </div>
            ))}
          </div>
        </section>
      )}
    </PageContainer>
  )
}

function ActivityTooltip({ active, payload, label }: TooltipContentProps) {
  if (!active || !payload?.length) return null
  return (
    <div className="rounded-md border border-border bg-popover px-3 py-2 font-mono text-[11px] shadow-xl">
      <div className="mb-1 text-muted-foreground">{label}</div>
      {payload.map((entry) => (
        <div key={String(entry.dataKey)} className="flex items-center justify-between gap-6">
          <span className="flex items-center gap-1.5">
            <span className="size-1.5 rounded-[1px]" style={{ background: entry.color }} />
            {entry.name}
          </span>
          <span className="tabular-nums">{Number(entry.value ?? 0).toLocaleString()}</span>
        </div>
      ))}
    </div>
  )
}
