import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, Gauge, Pencil, Plus, Search, ShieldOff, Users } from "lucide-react"
import { useMemo, useState, type FormEvent } from "react"
import { toast } from "sonner"

import { ApplicationDisplay } from "@/components/ApplicationDisplay"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PageContainer, PageHeader } from "@/components/PageContainer"
import { ChannelTag } from "@/components/Telemetry"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { apiErrorMessage } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import {
  deleteAppPolicy,
  getStats,
  listAppPolicies,
  listSentinelApplications,
  upsertAppPolicy,
  type AppPolicy,
  type AppPolicyInput,
  type Channel,
  type SentinelApplication,
} from "@/lib/notify"
import { cn } from "@/lib/utils"

const APP_POLICIES_KEY = ["appPolicies"]
const CHANNELS: Channel[] = ["EMAIL", "SMS", "DISCORD"]
const USAGE_DAYS = 30

type Filter = "all" | "allowed" | "none"

type ApplicationEntry = {
  clientID: string
  application?: SentinelApplication
  policy?: AppPolicy
  messages: number
}

export default function ApplicationsPage() {
  const { isAdmin, isLoading: authLoading } = useAuth()
  const [search, setSearch] = useState("")
  const [filter, setFilter] = useState<Filter>("all")
  const [editing, setEditing] = useState<ApplicationEntry | null>(null)

  const applications = useQuery({
    queryKey: ["sentinelApplications"],
    queryFn: listSentinelApplications,
    enabled: isAdmin,
  })
  const policies = useQuery({ queryKey: APP_POLICIES_KEY, queryFn: listAppPolicies, enabled: isAdmin })
  const stats = useQuery({ queryKey: ["stats", USAGE_DAYS], queryFn: () => getStats(USAGE_DAYS), enabled: isAdmin })

  const entries = useMemo(() => {
    const policyByClient = new Map((policies.data ?? []).map((policy) => [policy.client_id, policy]))
    const usageByClient = new Map((stats.data?.applications ?? []).map((usage) => [usage.client_id, usage.messages]))
    const list: ApplicationEntry[] = (applications.data ?? []).map((application) => ({
      clientID: application.client_id,
      application,
      policy: policyByClient.get(application.client_id),
      messages: usageByClient.get(application.client_id) ?? 0,
    }))
    const known = new Set(list.map((entry) => entry.clientID))
    for (const policy of policies.data ?? []) {
      if (!known.has(policy.client_id)) {
        list.push({ clientID: policy.client_id, policy, messages: usageByClient.get(policy.client_id) ?? 0 })
      }
    }
    return list.sort(
      (a, b) =>
        Number(!!b.policy) - Number(!!a.policy) ||
        (a.application?.name ?? a.clientID).localeCompare(b.application?.name ?? b.clientID),
    )
  }, [applications.data, policies.data, stats.data])

  const query = search.trim().toLowerCase()
  const visible = entries.filter((entry) => {
    if (filter === "allowed" && !entry.policy) return false
    if (filter === "none" && entry.policy) return false
    if (!query) return true
    return [entry.application?.name, entry.application?.description, entry.clientID].some((value) =>
      value?.toLowerCase().includes(query),
    )
  })
  const allowedCount = entries.filter((entry) => entry.policy).length

  if (!authLoading && !isAdmin) {
    return (
      <PageContainer>
        <PageHeader eyebrow="System" title="Applications" />
        <p className="text-sm text-muted-foreground">Only Notify admins can manage application access.</p>
      </PageContainer>
    )
  }

  const loading = authLoading || applications.isLoading || policies.isLoading
  const error = applications.error ?? policies.error

  return (
    <PageContainer>
      <PageHeader
        eyebrow="System"
        title="Applications"
        description="Sentinel applications allowed to send through Notify. Apps without access are rejected, and each one only reaches people on the channels you allow."
      />

      {error && (
        <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {apiErrorMessage(error, "Failed to load applications.")}
        </div>
      )}

      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search by name or client ID"
            className="pl-8"
            autoComplete="off"
          />
        </div>
        <FilterTabs
          value={filter}
          onChange={setFilter}
          options={[
            ["all", `All ${entries.length}`],
            ["allowed", `Allowed ${allowedCount}`],
            ["none", `No access ${entries.length - allowedCount}`],
          ]}
        />
      </div>

      <div className="space-y-3">
        {loading && <Skeleton className="h-24 w-full" />}
        {!loading && visible.length === 0 && (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
              {entries.length === 0 ? "Sentinel returned no applications." : "No applications match."}
            </CardContent>
          </Card>
        )}
        {visible.map((entry) => (
          <ApplicationRow key={entry.clientID} entry={entry} onEdit={() => setEditing(entry)} />
        ))}
      </div>

      <PolicyDialog
        key={editing?.clientID ?? "closed"}
        entry={editing}
        open={editing !== null}
        onClose={() => setEditing(null)}
      />
    </PageContainer>
  )
}

function ApplicationRow({ entry, onEdit }: { entry: ApplicationEntry; onEdit: () => void }) {
  const queryClient = useQueryClient()
  const { policy } = entry
  const name = entry.application?.name ?? entry.clientID
  const invalidate = () => queryClient.invalidateQueries({ queryKey: APP_POLICIES_KEY })

  const toggle = useMutation({
    mutationFn: (current: AppPolicy) => upsertAppPolicy(entry.clientID, { ...current, enabled: !current.enabled }),
    onSuccess: (saved) => {
      toast.success(saved.enabled ? `Enabled ${name}` : `Disabled ${name}`)
      return invalidate()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to update access.")),
  })
  const revoke = useMutation({
    mutationFn: () => deleteAppPolicy(entry.clientID),
    onSuccess: () => {
      toast.success(`Revoked ${name}`)
      return invalidate()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to revoke access.")),
  })

  async function copyClientID() {
    try {
      await navigator.clipboard.writeText(entry.clientID)
      toast.success("Client ID copied")
    } catch {
      toast.error("Could not copy to clipboard")
    }
  }

  return (
    <Card className={cn(policy && !policy.enabled && "opacity-60")}>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center">
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <ApplicationDisplay clientID={entry.clientID} application={entry.application} showClientID={false} />
            {!entry.application && <Badge variant="outline">Not in Sentinel</Badge>}
            {policy && !policy.enabled && <Badge variant="destructive">Disabled</Badge>}
          </div>

          {policy ? (
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
              {policy.allowed_channels.length > 0 ? (
                policy.allowed_channels.map((channel) => <ChannelTag key={channel} channel={channel} />)
              ) : (
                <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">No channels</span>
              )}
              {policy.can_target_groups && (
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                  <Users className="size-3" />
                  Groups
                </span>
              )}
              <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                <Gauge className="size-3" />
                {policy.daily_limit > 0 ? `${policy.daily_limit.toLocaleString()} deliveries/day` : "No daily limit"}
              </span>
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">No access. Sends from this app are rejected.</p>
          )}

          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <button type="button" onClick={copyClientID} className="inline-flex items-center gap-1 font-mono hover:text-foreground">
              {entry.clientID}
              <Copy className="size-3" />
            </button>
            {entry.messages > 0 && (
              <span>
                {entry.messages.toLocaleString()} message{entry.messages === 1 ? "" : "s"} · {USAGE_DAYS}d
              </span>
            )}
          </div>
          {policy?.description && <p className="text-sm text-muted-foreground">{policy.description}</p>}
        </div>

        <div className="flex flex-wrap gap-2">
          {policy ? (
            <>
              <Button variant="outline" size="sm" disabled={toggle.isPending} onClick={() => toggle.mutate(policy)}>
                {policy.enabled ? "Disable" : "Enable"}
              </Button>
              <Button variant="outline" size="sm" onClick={onEdit}>
                <Pencil className="size-3.5" />
                Edit
              </Button>
              <ConfirmDialog
                trigger={
                  <Button variant="destructive" size="sm">
                    <ShieldOff className="size-3.5" />
                    Revoke
                  </Button>
                }
                title={`Revoke ${name}?`}
                description="New sends from this app will be rejected. Messages already queued still go out."
                confirmLabel="Revoke"
                isPending={revoke.isPending}
                onConfirm={async () => {
                  await revoke.mutateAsync()
                }}
              />
            </>
          ) : (
            entry.application && (
              <Button variant="outline" size="sm" onClick={onEdit}>
                <Plus className="size-3.5" />
                Grant access
              </Button>
            )
          )}
        </div>
      </CardContent>
    </Card>
  )
}

function PolicyDialog({ entry, open, onClose }: { entry: ApplicationEntry | null; open: boolean; onClose: () => void }) {
  const queryClient = useQueryClient()
  const policy = entry?.policy
  const [channels, setChannels] = useState<Channel[]>(policy?.allowed_channels ?? [])
  const [canTargetGroups, setCanTargetGroups] = useState(policy?.can_target_groups ?? false)
  const [dailyLimit, setDailyLimit] = useState(String(policy?.daily_limit ?? 0))
  const [description, setDescription] = useState(policy?.description ?? "")
  const name = entry?.application?.name ?? entry?.clientID ?? ""

  const save = useMutation({
    mutationFn: () => {
      const input: AppPolicyInput = {
        description: description.trim(),
        allowed_channels: CHANNELS.filter((channel) => channels.includes(channel)),
        can_target_groups: canTargetGroups,
        daily_limit: Number(dailyLimit) || 0,
        enabled: policy?.enabled ?? true,
      }
      return upsertAppPolicy(entry!.clientID, input)
    },
    onSuccess: async () => {
      toast.success(policy ? `Updated ${name}` : `Granted ${name} access`)
      await queryClient.invalidateQueries({ queryKey: APP_POLICIES_KEY })
      onClose()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to save access.")),
  })

  function toggleChannel(channel: Channel) {
    setChannels((current) => (current.includes(channel) ? current.filter((c) => c !== channel) : [...current, channel]))
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (channels.length === 0) {
      toast.error("Allow at least one channel.")
      return
    }
    const limit = Number(dailyLimit)
    if (!Number.isInteger(limit) || limit < 0) {
      toast.error("Daily limit must be a whole number, 0 for unlimited.")
      return
    }
    save.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <form onSubmit={handleSubmit} className="space-y-5">
          <DialogHeader>
            <DialogTitle>{policy ? `Edit ${name}` : `Grant ${name} access`}</DialogTitle>
            <DialogDescription>
              Tokens issued to this app, including its service accounts, can send on the channels below.
            </DialogDescription>
          </DialogHeader>

          {entry && (
            <div className="rounded-md border border-border bg-muted/30 px-3 py-2">
              <ApplicationDisplay clientID={entry.clientID} application={entry.application} />
            </div>
          )}

          <div className="space-y-2">
            <Label>Channels</Label>
            <div className="grid grid-cols-3 gap-2">
              {CHANNELS.map((channel) => {
                const selected = channels.includes(channel)
                return (
                  <button
                    key={channel}
                    type="button"
                    aria-pressed={selected}
                    onClick={() => toggleChannel(channel)}
                    className={cn(
                      "flex h-10 items-center justify-center rounded-md border transition-colors",
                      selected ? "border-primary bg-primary/10" : "border-border bg-card/60 opacity-60 hover:opacity-100",
                    )}
                  >
                    <ChannelTag channel={channel} />
                  </button>
                )
              })}
            </div>
          </div>

          <label className="flex cursor-pointer items-start gap-3 rounded-md border border-border px-3 py-2.5">
            <input
              type="checkbox"
              checked={canTargetGroups}
              onChange={(event) => setCanTargetGroups(event.target.checked)}
              className="mt-0.5 size-4 accent-primary"
            />
            <span className="space-y-0.5">
              <span className="block text-sm font-medium">Can message groups</span>
              <span className="block text-xs text-muted-foreground">
                Lets the app send to whole Sentinel groups instead of only individual people.
              </span>
            </span>
          </label>

          <div className="space-y-2">
            <Label htmlFor="policy-limit">Daily delivery limit</Label>
            <Input
              id="policy-limit"
              type="number"
              min={0}
              step={1}
              value={dailyLimit}
              onChange={(event) => setDailyLimit(event.target.value)}
            />
            <p className="text-xs text-muted-foreground">Deliveries in a rolling 24 hours. 0 means unlimited.</p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="policy-description">Description</Label>
            <Textarea
              id="policy-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              maxLength={500}
              placeholder="What this app sends and why"
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? "Saving" : policy ? "Save" : "Grant access"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function FilterTabs<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T
  options: [T, string][]
  onChange: (value: T) => void
}) {
  return (
    <div className="inline-flex h-9 shrink-0 rounded-md border border-border bg-card/60 p-0.5 font-mono text-[11px]">
      {options.map(([option, label]) => (
        <button
          key={option}
          type="button"
          onClick={() => onChange(option)}
          className={cn(
            "rounded-sm px-3 uppercase tracking-[0.1em] transition-colors",
            value === option ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {label}
        </button>
      ))}
    </div>
  )
}
