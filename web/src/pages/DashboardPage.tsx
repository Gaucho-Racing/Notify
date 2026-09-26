import { useQuery } from "@tanstack/react-query"

import { PageContainer, PageHeader } from "@/components/PageContainer"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { getStats } from "@/lib/notify"

const STATS_WINDOW_DAYS = 30

export default function DashboardPage() {
  const { data: stats, isLoading, error } = useQuery({
    queryKey: ["stats", STATS_WINDOW_DAYS],
    queryFn: () => getStats(STATS_WINDOW_DAYS),
  })

  const tiles = [
    { label: "Messages", value: stats?.messages },
    { label: "Sent", value: stats?.deliveries.sent },
    { label: "Failed", value: stats?.deliveries.failed },
    { label: "Skipped", value: stats?.deliveries.skipped },
    { label: "Pending", value: stats?.deliveries.pending },
  ]

  return (
    <PageContainer>
      <PageHeader title="Dashboard" description={`Delivery activity over the last ${STATS_WINDOW_DAYS} days.`} />

      {error && (
        <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Failed to load stats.
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {tiles.map((tile) => (
          <Card key={tile.label}>
            <CardHeader>
              <CardDescription>{tile.label}</CardDescription>
              <CardTitle className="text-2xl tabular-nums">
                {isLoading ? <Skeleton className="h-7 w-16" /> : (tile.value ?? 0).toLocaleString()}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      {!!stats?.top_errors.length && (
        <Card className="mt-6">
          <CardHeader>
            <CardTitle>Top errors</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            {stats.top_errors.map((row) => (
              <div key={`${row.channel}-${row.error}`} className="flex items-start justify-between gap-4">
                <span className="min-w-0 break-words font-mono text-xs">
                  <span className="text-muted-foreground">{row.channel}</span> {row.error}
                </span>
                <span className="shrink-0 tabular-nums">{row.count}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </PageContainer>
  )
}
