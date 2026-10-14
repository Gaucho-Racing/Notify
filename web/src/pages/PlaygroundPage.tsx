import { useMutation, useQuery } from "@tanstack/react-query"
import { Copy, Search, Send, UserRound, Users, X } from "lucide-react"
import { useMemo, useState, type FormEvent, type ReactNode } from "react"
import { toast } from "sonner"

import { IdentityDisplay } from "@/components/IdentityDisplay"
import { PageContainer, PageHeader } from "@/components/PageContainer"
import { ChannelTag, Eyebrow, SignalDot } from "@/components/Telemetry"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { apiErrorMessage } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useIdentityDirectory } from "@/lib/directory"
import {
  getMessage,
  listDiscordBots,
  listEmailSenders,
  listGroups,
  listSMSSenders,
  listUsers,
  sendMessage,
  type Channel,
  type DeliveryStatus,
  type SendMessageInput,
} from "@/lib/notify"
import { cn } from "@/lib/utils"

const DEFAULT_SENDER = "__default"
const MAX_SEARCH_RESULTS = 6
const POLL_INTERVAL_MS = 2000

type SenderOption = { id: string; name: string; detail: string; isDefault: boolean; enabled: boolean }

const SENDER_LABELS: Record<Channel, string> = {
  EMAIL: "Email sender",
  SMS: "SMS sender",
  DISCORD: "Discord bot",
}
const CHANNEL_ORDER: Channel[] = ["EMAIL", "SMS", "DISCORD"]

const STATUS_SIGNAL: Record<DeliveryStatus, "live" | "down" | "warn" | "idle"> = {
  SENT: "live",
  FAILED: "down",
  PENDING: "warn",
  SENDING: "warn",
  SKIPPED: "idle",
}

export default function PlaygroundPage() {
  const { isAdmin, isLoading: authLoading, session } = useAuth()
  const [entityIDs, setEntityIDs] = useState<string[]>([])
  const [groupIDs, setGroupIDs] = useState<string[]>([])
  const [channels, setChannels] = useState<Channel[]>(["EMAIL"])
  const [senders, setSenders] = useState<Record<Channel, string>>({ EMAIL: DEFAULT_SENDER, SMS: DEFAULT_SENDER, DISCORD: DEFAULT_SENDER })
  const [subject, setSubject] = useState("")
  const [body, setBody] = useState("")
  const [actionURL, setActionURL] = useState("")
  const [actionLabel, setActionLabel] = useState("")
  const [category, setCategory] = useState("playground")
  const [messageID, setMessageID] = useState<string | null>(null)

  const users = useQuery({ queryKey: ["sentinelUsers"], queryFn: listUsers, enabled: isAdmin, staleTime: 5 * 60 * 1000 })
  const groups = useQuery({ queryKey: ["sentinelGroups"], queryFn: listGroups, enabled: isAdmin, staleTime: 5 * 60 * 1000 })
  const emailSenders = useQuery({ queryKey: ["emailSenders"], queryFn: listEmailSenders, enabled: isAdmin })
  const smsSenders = useQuery({ queryKey: ["smsSenders"], queryFn: listSMSSenders, enabled: isAdmin })
  const discordBots = useQuery({ queryKey: ["discordBots"], queryFn: listDiscordBots, enabled: isAdmin })

  const senderOptions: Record<Channel, SenderOption[]> = {
    EMAIL: (emailSenders.data ?? []).map((sender) => ({
      id: sender.id,
      name: sender.name,
      detail: sender.from_address,
      isDefault: sender.is_default,
      enabled: sender.enabled,
    })),
    SMS: (smsSenders.data ?? []).map((sender) => ({
      id: sender.id,
      name: sender.name,
      detail: sender.from_number || "Messaging service",
      isDefault: sender.is_default,
      enabled: sender.enabled,
    })),
    DISCORD: (discordBots.data ?? []).map((bot) => ({
      id: bot.id,
      name: bot.name,
      detail: `@${bot.discord_username}`,
      isDefault: bot.is_default,
      enabled: bot.enabled,
    })),
  }

  const payload = useMemo<SendMessageInput>(() => {
    const input: SendMessageInput = {
      recipients: { entity_ids: entityIDs, group_ids: groupIDs },
      channels: CHANNEL_ORDER.filter((channel) => channels.includes(channel)),
      subject: subject.trim(),
      body: body.trim(),
    }
    const chosenSender = (channel: Channel) =>
      input.channels.includes(channel) && senders[channel] !== DEFAULT_SENDER ? senders[channel] : undefined
    input.email_sender_id = chosenSender("EMAIL")
    input.sms_sender_id = chosenSender("SMS")
    input.discord_bot_id = chosenSender("DISCORD")
    if (actionURL.trim()) input.action_url = actionURL.trim()
    if (actionLabel.trim()) input.action_label = actionLabel.trim()
    if (category.trim()) input.category = category.trim()
    return input
  }, [entityIDs, groupIDs, channels, senders, subject, body, actionURL, actionLabel, category])

  const send = useMutation({
    mutationFn: () => sendMessage(payload),
    onSuccess: (message) => {
      setMessageID(message.id)
      toast.success(`Queued ${message.id}`)
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to send.")),
  })

  if (!authLoading && !isAdmin) {
    return (
      <PageContainer>
        <PageHeader title="Playground" />
        <p className="text-sm text-muted-foreground">Only Notify admins can send from the playground.</p>
      </PageContainer>
    )
  }

  const recipientCount = entityIDs.length + groupIDs.length
  const canSend = recipientCount > 0 && channels.length > 0 && payload.subject !== "" && payload.body !== "" && !send.isPending

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (canSend) send.mutate()
  }

  function toggleChannel(channel: Channel) {
    setChannels((current) => (current.includes(channel) ? current.filter((value) => value !== channel) : [...current, channel]))
  }

  return (
    <PageContainer>
      <PageHeader
        eyebrow="Overview"
        title="Playground"
        description="Send a real notification through Notify and watch each delivery land. Messages are sent as you, from Notify's own client."
      />

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.85fr)]">
        <form onSubmit={handleSubmit} className="space-y-4">
          <Panel label="01 · Recipients">
            <RecipientPicker
              users={users.data ?? []}
              usersError={users.error}
              selected={entityIDs}
              onChange={setEntityIDs}
              selfEntityID={session?.entity_id}
            />
            <div className="mt-4 space-y-2">
              <Label className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Users className="size-3.5" />
                Groups
              </Label>
              <div className="flex flex-wrap gap-1.5">
                {(groups.data ?? []).map((group) => {
                  const active = groupIDs.includes(group.id)
                  return (
                    <button
                      key={group.id}
                      type="button"
                      onClick={() =>
                        setGroupIDs((current) => (active ? current.filter((id) => id !== group.id) : [...current, group.id]))
                      }
                      className={cn(
                        "rounded-sm border px-2 py-1 text-xs transition-colors",
                        active ? "border-primary/50 bg-accent text-foreground" : "border-border text-muted-foreground hover:text-foreground",
                      )}
                    >
                      {group.name}
                      <span className="ml-1.5 font-mono text-[10px] text-muted-foreground">{group.member_count}</span>
                    </button>
                  )
                })}
                {groups.error && <p className="text-xs text-destructive">{apiErrorMessage(groups.error, "Failed to load groups.")}</p>}
              </div>
            </div>
          </Panel>

          <Panel label="02 · Channels">
            <div className="grid gap-2 sm:grid-cols-3">
              {CHANNEL_ORDER.map((channel) => {
                const active = channels.includes(channel)
                return (
                  <button
                    key={channel}
                    type="button"
                    onClick={() => toggleChannel(channel)}
                    className={cn(
                      "flex items-center justify-between rounded-md border px-3 py-2.5 text-left transition-colors",
                      active ? "border-primary/50 bg-accent" : "border-border hover:border-border hover:bg-muted/50",
                    )}
                  >
                    <ChannelTag channel={channel} />
                    <span className={cn("size-3 rounded-[2px] border", active ? "border-primary bg-primary" : "border-muted-foreground/40")} />
                  </button>
                )
              })}
            </div>
            {channels.length > 0 && (
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                {CHANNEL_ORDER.filter((channel) => channels.includes(channel)).map((channel) => {
                  const options = senderOptions[channel]
                  const fallback = options.find((option) => option.isDefault)
                  return (
                    <div key={channel} className="space-y-1.5">
                      <Label className="text-xs text-muted-foreground">{SENDER_LABELS[channel]}</Label>
                      <Select value={senders[channel]} onValueChange={(value) => setSenders((current) => ({ ...current, [channel]: value }))}>
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={DEFAULT_SENDER}>Default{fallback ? ` · ${fallback.name}` : " (none set)"}</SelectItem>
                          {options
                            .filter((option) => option.enabled)
                            .map((option) => (
                              <SelectItem key={option.id} value={option.id}>
                                {option.name} · {option.detail}
                              </SelectItem>
                            ))}
                        </SelectContent>
                      </Select>
                    </div>
                  )
                })}
              </div>
            )}
          </Panel>

          <Panel label="03 · Message">
            <div className="space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="pg-subject" className="text-xs text-muted-foreground">Subject</Label>
                <Input id="pg-subject" value={subject} onChange={(event) => setSubject(event.target.value)} maxLength={200} placeholder="Track day moved to Saturday" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pg-body" className="text-xs text-muted-foreground">Body</Label>
                <Textarea
                  id="pg-body"
                  value={body}
                  onChange={(event) => setBody(event.target.value)}
                  maxLength={10000}
                  rows={6}
                  placeholder={"Blank lines start new paragraphs in email.\nSMS joins it into one text."}
                />
              </div>
              <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_140px]">
                <div className="space-y-1.5">
                  <Label htmlFor="pg-action-url" className="text-xs text-muted-foreground">Action URL</Label>
                  <Input id="pg-action-url" type="url" value={actionURL} onChange={(event) => setActionURL(event.target.value)} placeholder="https://" />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="pg-action-label" className="text-xs text-muted-foreground">Action label</Label>
                  <Input id="pg-action-label" value={actionLabel} onChange={(event) => setActionLabel(event.target.value)} maxLength={40} placeholder="Open" />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pg-category" className="text-xs text-muted-foreground">Category</Label>
                <Input id="pg-category" value={category} onChange={(event) => setCategory(event.target.value)} maxLength={64} className="font-mono" />
              </div>
            </div>
          </Panel>

          <div className="flex flex-wrap items-center justify-between gap-3">
            <span className="font-mono text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
              {entityIDs.length} {entityIDs.length === 1 ? "person" : "people"} · {groupIDs.length} {groupIDs.length === 1 ? "group" : "groups"} ·{" "}
              {channels.length} {channels.length === 1 ? "channel" : "channels"}
            </span>
            <Button type="submit" disabled={!canSend}>
              <Send className="size-4" />
              {send.isPending ? "Sending" : "Send notification"}
            </Button>
          </div>
        </form>

        <div className="space-y-4 lg:sticky lg:top-20 lg:self-start">
          {messageID ? <TransmissionLog messageID={messageID} onClear={() => setMessageID(null)} /> : <Preview subject={subject} body={body} actionURL={actionURL} actionLabel={actionLabel} channels={channels} />}
          <RequestPanel payload={payload} />
        </div>
      </div>
    </PageContainer>
  )
}

function Panel({ label, action, children }: { label: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="rounded-lg border border-border bg-card/90">
      <div className="flex items-center justify-between border-b border-border px-5 py-3">
        <Eyebrow>{label}</Eyebrow>
        {action}
      </div>
      <div className="p-5">{children}</div>
    </section>
  )
}

function RecipientPicker({
  users,
  usersError,
  selected,
  onChange,
  selfEntityID,
}: {
  users: { entity_id: string; name: string; username: string; avatar_url: string }[]
  usersError: unknown
  selected: string[]
  onChange: (ids: string[]) => void
  selfEntityID?: string
}) {
  const [search, setSearch] = useState("")
  const byID = useMemo(() => new Map(users.map((user) => [user.entity_id, user])), [users])
  const query = search.trim().toLowerCase()
  const results = query
    ? users
        .filter((user) => !selected.includes(user.entity_id))
        .filter((user) => user.name.toLowerCase().includes(query) || user.username.toLowerCase().includes(query))
        .slice(0, MAX_SEARCH_RESULTS)
    : []

  function add(entityID: string) {
    if (!selected.includes(entityID)) onChange([...selected, entityID])
    setSearch("")
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label htmlFor="pg-search" className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <UserRound className="size-3.5" />
          People
        </Label>
        {selfEntityID && !selected.includes(selfEntityID) && (
          <button type="button" onClick={() => add(selfEntityID)} className="font-mono text-[10px] uppercase tracking-[0.12em] text-primary hover:underline">
            + Add me
          </button>
        )}
      </div>
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input id="pg-search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search by name or username" className="pl-8" autoComplete="off" />
        {results.length > 0 && (
          <div className="absolute inset-x-0 top-full z-20 mt-1 overflow-hidden rounded-md border border-border bg-popover shadow-xl">
            {results.map((user) => (
              <button
                key={user.entity_id}
                type="button"
                onClick={() => add(user.entity_id)}
                className="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm hover:bg-accent"
              >
                <Avatar size="sm">
                  {user.avatar_url && <AvatarImage src={user.avatar_url} alt="" />}
                  <AvatarFallback>{user.name.slice(0, 1).toUpperCase()}</AvatarFallback>
                </Avatar>
                <span className="min-w-0 truncate">{user.name}</span>
                <span className="truncate font-mono text-[11px] text-muted-foreground">@{user.username}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      {!!usersError && <p className="text-xs text-destructive">{apiErrorMessage(usersError, "Failed to load people.")}</p>}
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1.5 pt-1">
          {selected.map((entityID) => {
            const user = byID.get(entityID)
            return (
              <span key={entityID} className="inline-flex items-center gap-1.5 rounded-sm border border-border bg-muted/60 py-0.5 pr-1 pl-2 text-xs">
                {user?.name ?? entityID}
                {entityID === selfEntityID && <span className="font-mono text-[10px] text-muted-foreground">you</span>}
                <button type="button" onClick={() => onChange(selected.filter((id) => id !== entityID))} aria-label="Remove" className="rounded-sm p-0.5 text-muted-foreground hover:text-foreground">
                  <X className="size-3" />
                </button>
              </span>
            )
          })}
        </div>
      )}
    </div>
  )
}

function Preview({
  subject,
  body,
  actionURL,
  actionLabel,
  channels,
}: {
  subject: string
  body: string
  actionURL: string
  actionLabel: string
  channels: Channel[]
}) {
  const label = actionLabel.trim() || "Open"
  const shownSubject = subject.trim() || "Subject"
  const shownBody = body.trim() || "Your message body shows up here."

  return (
    <Panel label="Preview">
      {channels.length === 0 && <p className="text-sm text-muted-foreground">Pick a channel to preview it.</p>}
      <div className="space-y-5">
        {channels.includes("EMAIL") && (
          <div className="space-y-2">
            <ChannelTag channel="EMAIL" />
            <div className="rounded-md bg-[#f4f4f5] p-4 text-[#18181b]">
              <div className="rounded-md bg-white p-5">
                <div className="mb-3 text-base font-semibold">{shownSubject}</div>
                {shownBody.split(/\n\s*\n/).map((paragraph, index) => (
                  <p key={index} className="mb-2 text-sm leading-relaxed whitespace-pre-line text-[#3f3f46]">{paragraph}</p>
                ))}
                {actionURL.trim() && <span className="mt-2 inline-block rounded-md bg-[#18181b] px-3.5 py-2 text-xs font-semibold text-white">{label}</span>}
              </div>
            </div>
          </div>
        )}
        {channels.includes("SMS") && (
          <div className="space-y-2">
            <ChannelTag channel="SMS" />
            <div className="flex justify-start">
              <div className="max-w-[85%] rounded-2xl rounded-bl-sm bg-[#26262b] px-3.5 py-2.5 text-sm whitespace-pre-line text-[#f4f4f5]">
                {`Notify: ${shownSubject}\n\n${shownBody}${actionURL.trim() ? `\n\n${label}: ${actionURL.trim()}` : ""}`}
              </div>
            </div>
          </div>
        )}
        {channels.includes("DISCORD") && (
          <div className="space-y-2">
            <ChannelTag channel="DISCORD" />
            <div className="rounded-md bg-[#313338] p-3">
              <div className="rounded-sm border-l-4 border-[#18181b] bg-[#2b2d31] px-3.5 py-3">
                <div className={cn("mb-1 text-sm font-semibold", actionURL.trim() ? "text-[#00a8fc]" : "text-[#f2f3f5]")}>{shownSubject}</div>
                <div className="text-sm whitespace-pre-line text-[#dbdee1]">
                  {shownBody}
                  {actionURL.trim() && (
                    <>
                      {"\n\n"}
                      <span className="text-[#00a8fc]">{label}</span>
                    </>
                  )}
                </div>
                <div className="mt-2 text-[11px] text-[#949ba4]">Sent by Notify</div>
              </div>
            </div>
          </div>
        )}
      </div>
    </Panel>
  )
}

function TransmissionLog({ messageID, onClear }: { messageID: string; onClear: () => void }) {
  const detail = useQuery({
    queryKey: ["playgroundMessage", messageID],
    queryFn: () => getMessage(messageID),
    refetchInterval: (query) => {
      const deliveries = query.state.data?.deliveries ?? []
      const settled = deliveries.length > 0 && deliveries.every((delivery) => delivery.status !== "PENDING" && delivery.status !== "SENDING")
      return settled ? false : POLL_INTERVAL_MS
    },
  })
  const deliveries = detail.data?.deliveries ?? []
  const directory = useIdentityDirectory(deliveries.map((delivery) => delivery.entity_id))
  const inFlight = deliveries.some((delivery) => delivery.status === "PENDING" || delivery.status === "SENDING")

  return (
    <Panel
      label="Transmission"
      action={
        <button type="button" onClick={onClear} className="font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground hover:text-foreground">
          Back to preview
        </button>
      }
    >
      <div className="mb-4 flex items-center justify-between gap-3">
        <span className="truncate font-mono text-xs text-muted-foreground">{messageID}</span>
        <span className="flex shrink-0 items-center gap-2 font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
          <SignalDot state={inFlight || detail.isLoading ? "warn" : "live"} pulse={inFlight || detail.isLoading} />
          {inFlight || detail.isLoading ? "In flight" : "Settled"}
        </span>
      </div>
      {detail.error && <p className="text-sm text-destructive">{apiErrorMessage(detail.error, "Failed to load deliveries.")}</p>}
      <div className="divide-y divide-border rounded-md border border-border">
        {deliveries.map((delivery) => (
          <div key={delivery.id} className="space-y-1 px-3 py-2.5">
            <div className="flex items-center justify-between gap-3">
              <IdentityDisplay
                entityID={delivery.entity_id}
                identity={directory.byID.get(delivery.entity_id)}
                loading={directory.isLoading}
                size="sm"
                showDetails={false}
              />
              <span className="flex shrink-0 items-center gap-3">
                <ChannelTag channel={delivery.channel} />
                <span className="flex w-20 items-center justify-end gap-1.5 font-mono text-[10px] uppercase tracking-[0.12em]">
                  <SignalDot state={STATUS_SIGNAL[delivery.status]} pulse={delivery.status === "PENDING" || delivery.status === "SENDING"} />
                  {delivery.status}
                </span>
              </span>
            </div>
            {(delivery.destination || delivery.last_error) && (
              <div className="pl-8 font-mono text-[11px] break-words text-muted-foreground">
                {delivery.destination}
                {delivery.last_error && (
                  <span className={cn("block", delivery.status === "FAILED" ? "text-status-failed" : "text-muted-foreground")}>
                    {delivery.last_error}
                  </span>
                )}
              </div>
            )}
          </div>
        ))}
        {!detail.isLoading && deliveries.length === 0 && <p className="px-3 py-4 text-sm text-muted-foreground">No deliveries.</p>}
      </div>
    </Panel>
  )
}

function RequestPanel({ payload }: { payload: SendMessageInput }) {
  const json = JSON.stringify(payload, null, 2)

  async function copy() {
    try {
      await navigator.clipboard.writeText(json)
      toast.success("Request body copied")
    } catch {
      toast.error("Could not copy to clipboard")
    }
  }

  return (
    <Panel
      label="POST /api/messages"
      action={
        <button type="button" onClick={copy} className="text-muted-foreground hover:text-foreground" aria-label="Copy request body">
          <Copy className="size-3.5" />
        </button>
      }
    >
      <pre className="max-h-72 overflow-auto font-mono text-[11px] leading-relaxed text-foreground/85">{json}</pre>
    </Panel>
  )
}
