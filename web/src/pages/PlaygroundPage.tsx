import { useMutation, useQuery } from "@tanstack/react-query"
import { Copy, FileUp, Hash, Megaphone, Search, Send, UserRound, Users, X } from "lucide-react"
import { useMemo, useRef, useState, type ChangeEvent, type DragEvent, type FormEvent, type ReactNode } from "react"
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
  listDiscordBotChannels,
  listDiscordBots,
  listEmailSenders,
  listGroups,
  listSMSSenders,
  listUsers,
  sendMessage,
  type Channel,
  type DeliveryStatus,
  type DiscordEmbed,
  type DiscordGuild,
  type SendMessageInput,
} from "@/lib/notify"
import { cn } from "@/lib/utils"

const DEFAULT_SENDER = "__default"
const MAX_SEARCH_RESULTS = 6
const POLL_INTERVAL_MS = 2000
const SMS_MAX_LENGTH = 1600
const DISCORD_MAX_CONTENT = 2000
const EMAIL_HTML_MAX_BYTES = 100_000
const DEFAULT_EMBED_COLOR = "#e105a3"

type SenderOption = { id: string; name: string; detail: string; isDefault: boolean; enabled: boolean }

type EmbedDraft = { title: string; description: string; url: string; color: string; footer: string }
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
  const [channel, setChannel] = useState<Channel>("EMAIL")
  const [senders, setSenders] = useState<Record<Channel, string>>({ EMAIL: DEFAULT_SENDER, SMS: DEFAULT_SENDER, DISCORD: DEFAULT_SENDER })
  const [entityIDs, setEntityIDs] = useState<string[]>([])
  const [groupIDs, setGroupIDs] = useState<string[]>([])
  const [discordChannelIDs, setDiscordChannelIDs] = useState<string[]>([])
  const [subject, setSubject] = useState("")
  const [body, setBody] = useState("")
  const [category, setCategory] = useState("playground")
  const [emailFormat, setEmailFormat] = useState<"plain" | "html">("plain")
  const [emailHTML, setEmailHTML] = useState("")
  const [discordFormat, setDiscordFormat] = useState<"plain" | "rich">("plain")
  const [discordIdentity, setDiscordIdentity] = useState<"bot" | "custom">("bot")
  const [webhookName, setWebhookName] = useState("")
  const [webhookAvatar, setWebhookAvatar] = useState("")
  const [embed, setEmbed] = useState<EmbedDraft>({ title: "", description: "", url: "", color: DEFAULT_EMBED_COLOR, footer: "" })
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
  const selectedBotID = senders.DISCORD === DEFAULT_SENDER ? discordBots.data?.find((bot) => bot.is_default)?.id : senders.DISCORD
  const selectedBot = discordBots.data?.find((bot) => bot.id === selectedBotID)
  const botChannels = useQuery({
    queryKey: ["discordBotChannels", selectedBotID],
    queryFn: () => listDiscordBotChannels(selectedBotID ?? ""),
    enabled: isAdmin && channel === "DISCORD" && !!selectedBotID,
    staleTime: 60 * 1000,
  })
  const { channelNames, noWebhookChannels } = useMemo(() => {
    const names = new Map<string, string>()
    const noWebhook = new Set<string>()
    for (const guild of botChannels.data ?? []) {
      for (const guildChannel of guild.channels) {
        names.set(guildChannel.id, guildChannel.name)
        if (!guildChannel.can_webhook) noWebhook.add(guildChannel.id)
      }
    }
    return { channelNames: names, noWebhookChannels: noWebhook }
  }, [botChannels.data])
  const customIdentity = channel === "DISCORD" && discordIdentity === "custom"
  const richEmbed = useMemo(() => embedFromDraft(embed), [embed])

  const payload = useMemo<SendMessageInput>(() => {
    const senderID = senders[channel] !== DEFAULT_SENDER ? senders[channel] : undefined
    const input: SendMessageInput = {
      channel,
      recipients: { entity_ids: entityIDs, group_ids: groupIDs },
      category: category.trim() || undefined,
    }
    if (channel === "EMAIL") {
      input.email = {
        sender_id: senderID,
        subject: subject.trim(),
        body: body.trim(),
        html: emailFormat === "html" ? emailHTML.trim() || undefined : undefined,
      }
    } else if (channel === "SMS") {
      input.sms = { sender_id: senderID, body: body.trim() }
    } else {
      input.recipients.discord_channel_ids = discordChannelIDs
      input.discord = {
        bot_id: senderID,
        content: body.trim() || undefined,
        embeds: discordFormat === "rich" && richEmbed ? [richEmbed] : undefined,
        webhook:
          discordIdentity === "custom"
            ? { username: webhookName.trim(), avatar_url: webhookAvatar.trim() || undefined }
            : undefined,
      }
    }
    return input
  }, [
    channel,
    senders,
    entityIDs,
    groupIDs,
    discordChannelIDs,
    subject,
    body,
    category,
    emailFormat,
    emailHTML,
    discordFormat,
    richEmbed,
    discordIdentity,
    webhookName,
    webhookAvatar,
  ])

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

  const trimmedBody = body.trim()
  const smsLength = trimmedBody.length
  const discordLength = [...trimmedBody].length
  const htmlBytes = new TextEncoder().encode(emailHTML).length
  const recipientCount = entityIDs.length + groupIDs.length + (channel === "DISCORD" ? discordChannelIDs.length : 0)
  const problems = [
    recipientCount === 0 && (channel === "DISCORD" ? "Pick a channel or person to message." : "Add at least one recipient."),
    channel === "EMAIL" && subject.trim() === "" && "Email needs a subject.",
    channel !== "DISCORD" && trimmedBody === "" && "Write a message.",
    channel === "EMAIL" && emailFormat === "html" && emailHTML.trim() === "" && "Paste, drop, or load the email HTML.",
    channel === "EMAIL" && emailFormat === "html" && htmlBytes > EMAIL_HTML_MAX_BYTES && "Email HTML is over 100 KB.",
    channel === "SMS" && smsLength > SMS_MAX_LENGTH && `Message is over SMS's ${SMS_MAX_LENGTH}-character limit.`,
    channel === "DISCORD" && discordLength > DISCORD_MAX_CONTENT && `Message is over Discord's ${DISCORD_MAX_CONTENT}-character limit.`,
    channel === "DISCORD" && discordFormat === "plain" && trimmedBody === "" && "Write a message.",
    channel === "DISCORD" && discordFormat === "rich" && !richEmbed && "Give the embed a title or description.",
    customIdentity && webhookName.trim() === "" && "Give the custom identity a name.",
    customIdentity && /discord|clyde/i.test(webhookName) && 'Custom names can\'t contain "discord" or "clyde".',
    customIdentity && (entityIDs.length > 0 || groupIDs.length > 0) && "Custom identities only post in channels; remove the DM recipients.",
    customIdentity && discordChannelIDs.some((id) => noWebhookChannels.has(id)) && "The bot can't use a custom identity in a selected channel (needs Manage Webhooks).",
  ].filter((problem): problem is string => typeof problem === "string")
  const canSend = problems.length === 0 && !send.isPending

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (canSend) send.mutate()
  }

  function changeBot(value: string) {
    setSenders((current) => ({ ...current, DISCORD: value }))
    setDiscordChannelIDs([])
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
          <Panel label="01 · Channel">
            <div className="grid gap-2 sm:grid-cols-3">
              {CHANNEL_ORDER.map((option) => (
                <button
                  key={option}
                  type="button"
                  onClick={() => setChannel(option)}
                  className={cn(
                    "flex items-center justify-between rounded-md border px-3 py-2.5 text-left transition-colors",
                    channel === option ? "border-primary/50 bg-accent" : "border-border hover:bg-muted/50",
                  )}
                >
                  <ChannelTag channel={option} />
                  <span className={cn("size-3 rounded-full border", channel === option ? "border-primary bg-primary" : "border-muted-foreground/40")} />
                </button>
              ))}
            </div>
            <div className="mt-4">
              <SenderSelect
                label={channel === "DISCORD" ? "Bot" : "Sender"}
                value={senders[channel]}
                options={senderOptions[channel]}
                onChange={(value) => (channel === "DISCORD" ? changeBot(value) : setSenders((current) => ({ ...current, [channel]: value })))}
              />
            </div>
            {channel === "DISCORD" && (
              <div className="mt-4 space-y-3">
                <div className="flex items-center justify-between">
                  <Label className="text-xs text-muted-foreground">Send as</Label>
                  <Segmented
                    value={discordIdentity}
                    options={[["bot", "Bot"], ["custom", "Custom"]]}
                    onChange={(value) => setDiscordIdentity(value as "bot" | "custom")}
                  />
                </div>
                {discordIdentity === "custom" && (
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="space-y-1.5">
                      <Label htmlFor="pg-webhook-name" className="text-xs text-muted-foreground">Name</Label>
                      <Input id="pg-webhook-name" value={webhookName} onChange={(event) => setWebhookName(event.target.value)} maxLength={80} placeholder="Team Leads" />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="pg-webhook-avatar" className="text-xs text-muted-foreground">Avatar URL</Label>
                      <Input
                        id="pg-webhook-avatar"
                        type="url"
                        value={webhookAvatar}
                        onChange={(event) => setWebhookAvatar(event.target.value)}
                        placeholder="https://gauchoracing.com/logo/gr-logo-blank.png"
                      />
                    </div>
                    <p className="text-xs text-muted-foreground sm:col-span-2">
                      Posts through a temporary webhook that's deleted right after. Channel posts only; DMs always come from the bot.
                    </p>
                  </div>
                )}
              </div>
            )}
          </Panel>

          <Panel label="02 · Recipients">
            <div className="space-y-5">
              {channel === "DISCORD" && (
                <DiscordChannelPicker
                  needsWebhook={discordIdentity === "custom"}
                  guilds={botChannels.data ?? []}
                  loading={botChannels.isLoading}
                  error={botChannels.error}
                  hasBot={!!selectedBotID}
                  selected={discordChannelIDs}
                  onChange={setDiscordChannelIDs}
                />
              )}
              <div className="space-y-4">
                {channel === "DISCORD" && <Eyebrow className="block">Direct messages</Eyebrow>}
                <RecipientPicker
                  users={users.data ?? []}
                  usersError={users.error}
                  selected={entityIDs}
                  onChange={setEntityIDs}
                  selfEntityID={session?.entity_id}
                />
                <div className="space-y-2">
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
              </div>
            </div>
          </Panel>

          <Panel label="03 · Message">
            <div className="space-y-3">
              {channel === "EMAIL" && (
                <div className="space-y-1.5">
                  <Label htmlFor="pg-subject" className="text-xs text-muted-foreground">Subject</Label>
                  <Input id="pg-subject" value={subject} onChange={(event) => setSubject(event.target.value)} maxLength={200} placeholder="Track day moved to Saturday" />
                </div>
              )}
              {channel === "DISCORD" && (
                <div className="flex justify-end">
                  <Segmented
                    value={discordFormat}
                    options={[["plain", "Plain"], ["rich", "Rich"]]}
                    onChange={(value) => setDiscordFormat(value as "plain" | "rich")}
                  />
                </div>
              )}
              <div className="space-y-1.5">
                <Label htmlFor="pg-body" className="text-xs text-muted-foreground">
                  {channel === "EMAIL" ? "Body (plain text)" : channel === "DISCORD" && discordFormat === "rich" ? "Message above the embed (optional)" : "Message"}
                </Label>
                <Textarea
                  id="pg-body"
                  value={body}
                  onChange={(event) => setBody(event.target.value)}
                  maxLength={10000}
                  rows={channel === "SMS" ? 4 : 6}
                  placeholder={
                    channel === "DISCORD"
                      ? "Role and user mentions ping (<@&role-id>, <@user-id>). @everyone and @here never do."
                      : channel === "EMAIL"
                        ? "Sent as the plain-text email, and as the fallback when HTML is set."
                        : "Gaucho Racing: …"
                  }
                />
                {channel === "SMS" && <Meter label="Length" value={smsLength} max={SMS_MAX_LENGTH} />}
                {channel === "DISCORD" && <Meter label="Length" value={discordLength} max={DISCORD_MAX_CONTENT} />}
              </div>
              {channel === "EMAIL" && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs text-muted-foreground">Format</Label>
                    <Segmented
                      value={emailFormat}
                      options={[["plain", "Plain"], ["html", "HTML"]]}
                      onChange={(value) => setEmailFormat(value as "plain" | "html")}
                    />
                  </div>
                  {emailFormat === "html" && <HTMLInput value={emailHTML} bytes={htmlBytes} onChange={setEmailHTML} />}
                </div>
              )}
              {channel === "DISCORD" && discordFormat === "rich" && (
                <>
                  <p className="text-xs text-muted-foreground">Mentions only ping from the message above, not from inside the embed.</p>
                  <EmbedBuilder embed={embed} onChange={setEmbed} />
                </>
              )}
              <div className="space-y-1.5">
                <Label htmlFor="pg-category" className="text-xs text-muted-foreground">Category</Label>
                <Input id="pg-category" value={category} onChange={(event) => setCategory(event.target.value)} maxLength={64} className="font-mono" />
              </div>
            </div>
          </Panel>

          <div className="flex flex-wrap items-center justify-between gap-3">
            <span className={cn("font-mono text-[11px] tracking-[0.06em]", problems.length ? "text-muted-foreground" : "text-primary")}>
              {problems[0] ?? "Ready to send."}
            </span>
            <Button type="submit" disabled={!canSend}>
              <Send className="size-4" />
              {send.isPending ? "Sending" : "Send notification"}
            </Button>
          </div>
        </form>

        <div className="space-y-4 lg:sticky lg:top-20 lg:self-start">
          {messageID ? (
            <TransmissionLog messageID={messageID} channelNames={channelNames} onClear={() => setMessageID(null)} />
          ) : (
            <Preview
              channel={channel}
              subject={subject}
              body={body}
              emailHTML={emailFormat === "html" ? emailHTML : ""}
              discordEmbed={discordFormat === "rich" ? richEmbed : null}
              discordChannel={discordChannelIDs.length ? channelNames.get(discordChannelIDs[0]) : undefined}
              botName={customIdentity && webhookName.trim() ? webhookName.trim() : (selectedBot?.name ?? "Notify")}
              botAvatar={customIdentity ? webhookAvatar.trim() || undefined : selectedBot?.avatar_url}
            />
          )}
          <RequestPanel payload={payload} />
        </div>
      </div>
    </PageContainer>
  )
}

function embedFromDraft(draft: EmbedDraft): DiscordEmbed | null {
  const embed: DiscordEmbed = {}
  if (draft.title.trim()) embed.title = draft.title.trim()
  if (draft.description.trim()) embed.description = draft.description.trim()
  if (!embed.title && !embed.description) return null
  if (draft.url.trim()) embed.url = draft.url.trim()
  embed.color = parseInt(draft.color.slice(1), 16)
  if (draft.footer.trim()) embed.footer = { text: draft.footer.trim() }
  return embed
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

function SenderSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: SenderOption[]
  onChange: (value: string) => void
}) {
  const fallback = options.find((option) => option.isDefault)
  return (
    <div className="space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <Select value={value} onValueChange={onChange}>
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
}

function Segmented({ value, options, onChange }: { value: string; options: [string, string][]; onChange: (value: string) => void }) {
  return (
    <div className="inline-flex h-8 rounded-md border border-border bg-card/60 p-0.5 font-mono text-[11px]">
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

function Meter({ label, value, max }: { label: string; value: number; max: number }) {
  const over = value > max
  return (
    <div className="space-y-1">
      <div className="flex justify-between font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
        <span>{label}</span>
        <span className={cn("tabular-nums", over && "text-status-failed")}>
          {value.toLocaleString()} / {max.toLocaleString()}
        </span>
      </div>
      <div className="h-1 overflow-hidden rounded-full bg-muted">
        <div className={cn("h-full", over ? "bg-status-failed" : "bg-primary")} style={{ width: `${Math.min(value / max, 1) * 100}%` }} />
      </div>
    </div>
  )
}

function HTMLInput({ value, bytes, onChange }: { value: string; bytes: number; onChange: (value: string) => void }) {
  const fileInput = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)

  async function load(file: File) {
    if (!/\.html?$/i.test(file.name) && file.type !== "text/html") {
      toast.error("Drop an .html file.")
      return
    }
    if (file.size > EMAIL_HTML_MAX_BYTES) {
      toast.error("That file is over 100 KB.")
      return
    }
    onChange(await file.text())
    toast.success(`Loaded ${file.name}`)
  }

  function handlePick(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (file) void load(file)
  }

  function handleDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault()
    setDragging(false)
    const file = event.dataTransfer.files[0]
    if (file) void load(file)
  }

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <Label htmlFor="pg-html" className="text-xs text-muted-foreground">HTML</Label>
        <div className="flex items-center gap-3">
          <span className={cn("font-mono text-[10px] tabular-nums text-muted-foreground", bytes > EMAIL_HTML_MAX_BYTES && "text-status-failed")}>
            {(bytes / 1000).toFixed(1)} / 100 KB
          </span>
          <Button type="button" variant="outline" size="sm" onClick={() => fileInput.current?.click()}>
            <FileUp className="size-3.5" />
            Load .html
          </Button>
          <input ref={fileInput} type="file" accept=".html,.htm,text/html" className="hidden" onChange={handlePick} />
        </div>
      </div>
      <div
        className="relative"
        onDragOver={(event) => {
          if (!event.dataTransfer.types.includes("Files")) return
          event.preventDefault()
          event.dataTransfer.dropEffect = "copy"
          setDragging(true)
        }}
        onDragLeave={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragging(false)
        }}
        onDrop={handleDrop}
      >
        <Textarea
          id="pg-html"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          rows={8}
          className="font-mono text-xs"
          placeholder="Paste HTML or drop an .html file here. The body above is still sent as the plain-text version."
        />
        {dragging && (
          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-primary bg-background/90 text-sm">
            <FileUp className="size-5 text-primary" />
            Drop to load the HTML
          </div>
        )}
      </div>
    </div>
  )
}

function EmbedBuilder({ embed, onChange }: { embed: EmbedDraft; onChange: (embed: EmbedDraft) => void }) {
  const set = (field: keyof EmbedDraft) => (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    onChange({ ...embed, [field]: event.target.value })
  return (
    <div className="grid gap-3">
      <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_88px]">
        <div className="space-y-1.5">
          <Label htmlFor="pg-embed-title" className="text-xs text-muted-foreground">Title</Label>
          <Input id="pg-embed-title" value={embed.title} onChange={set("title")} maxLength={256} />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="pg-embed-color" className="text-xs text-muted-foreground">Color</Label>
          <Input id="pg-embed-color" type="color" value={embed.color} onChange={set("color")} className="p-1" />
        </div>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="pg-embed-description" className="text-xs text-muted-foreground">Description</Label>
        <Textarea id="pg-embed-description" value={embed.description} onChange={set("description")} maxLength={4096} rows={4} />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="pg-embed-url" className="text-xs text-muted-foreground">Title link</Label>
          <Input id="pg-embed-url" type="url" value={embed.url} onChange={set("url")} placeholder="https://" />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="pg-embed-footer" className="text-xs text-muted-foreground">Footer</Label>
          <Input id="pg-embed-footer" value={embed.footer} onChange={set("footer")} maxLength={2048} />
        </div>
      </div>
    </div>
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

function DiscordChannelPicker({
  needsWebhook,
  guilds,
  loading,
  error,
  hasBot,
  selected,
  onChange,
}: {
  needsWebhook: boolean
  guilds: DiscordGuild[]
  loading: boolean
  error: unknown
  hasBot: boolean
  selected: string[]
  onChange: (ids: string[]) => void
}) {
  const [search, setSearch] = useState("")
  const query = search.trim().toLowerCase().replace(/^#/, "")
  const byID = new Map(guilds.flatMap((guild) => guild.channels.map((guildChannel) => [guildChannel.id, guildChannel] as const)))

  function toggle(id: string) {
    onChange(selected.includes(id) ? selected.filter((value) => value !== id) : [...selected, id])
  }

  return (
    <div className="space-y-2">
      <Label htmlFor="pg-channel-search" className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Hash className="size-3.5" />
        Channels
      </Label>
      {!hasBot && <p className="text-sm text-muted-foreground">Add a Discord bot to post in channels.</p>}
      {hasBot && (
        <>
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input id="pg-channel-search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search channels" className="pl-8" autoComplete="off" />
          </div>
          {selected.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {selected.map((id) => (
                <span key={id} className="inline-flex items-center gap-1 rounded-sm border border-border bg-muted/60 py-0.5 pr-1 pl-2 text-xs">
                  #{byID.get(id)?.name ?? id}
                  <button type="button" onClick={() => toggle(id)} aria-label="Remove" className="rounded-sm p-0.5 text-muted-foreground hover:text-foreground">
                    <X className="size-3" />
                  </button>
                </span>
              ))}
            </div>
          )}
          <div className="max-h-64 overflow-y-auto rounded-md border border-border">
            {loading && <p className="px-3 py-3 text-sm text-muted-foreground">Loading channels…</p>}
            {!!error && <p className="px-3 py-3 text-sm text-destructive">{apiErrorMessage(error, "Failed to load channels.")}</p>}
            {!loading && !error && guilds.every((guild) => guild.channels.length === 0) && (
              <p className="px-3 py-3 text-sm text-muted-foreground">This bot can't post in any channels.</p>
            )}
            {guilds.map((guild) => {
              const channels = guild.channels.filter(
                (guildChannel) => !query || guildChannel.name.includes(query) || guildChannel.category?.toLowerCase().includes(query),
              )
              if (channels.length === 0) return null
              return (
                <div key={guild.id}>
                  <div className="sticky top-0 flex items-center gap-2 border-b border-border bg-card px-3 py-1.5">
                    {guild.icon_url && <img src={guild.icon_url} alt="" className="size-4 rounded-sm" />}
                    <Eyebrow>{guild.name}</Eyebrow>
                  </div>
                  {channels.map((guildChannel, index) => {
                    const active = selected.includes(guildChannel.id)
                    const newCategory = guildChannel.category && guildChannel.category !== channels[index - 1]?.category
                    return (
                      <div key={guildChannel.id}>
                        {newCategory && (
                          <div className="px-3 pt-2 pb-0.5 font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground/70">
                            {guildChannel.category}
                          </div>
                        )}
                        <button
                          type="button"
                          onClick={() => toggle(guildChannel.id)}
                          className={cn(
                            "flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm transition-colors",
                            active ? "bg-accent text-foreground" : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                          )}
                        >
                          {guildChannel.announcement ? <Megaphone className="size-3.5" /> : <Hash className="size-3.5" />}
                          <span className="min-w-0 flex-1 truncate">{guildChannel.name}</span>
                          {needsWebhook && !guildChannel.can_webhook && (
                            <span className="shrink-0 font-mono text-[10px] text-status-pending">no webhooks</span>
                          )}
                          <span className={cn("size-3 shrink-0 rounded-[2px] border", active ? "border-primary bg-primary" : "border-muted-foreground/40")} />
                        </button>
                      </div>
                    )
                  })}
                </div>
              )
            })}
          </div>
        </>
      )}
    </div>
  )
}

function Preview({
  channel,
  subject,
  body,
  emailHTML,
  discordEmbed,
  discordChannel,
  botName,
  botAvatar,
}: {
  channel: Channel
  subject: string
  body: string
  emailHTML: string
  discordEmbed: DiscordEmbed | null
  discordChannel?: string
  botName: string
  botAvatar?: string
}) {
  const shownBody = body.trim() || (channel === "DISCORD" && discordEmbed ? "" : "Your message shows up here.")

  return (
    <Panel label="Preview">
      <div className="space-y-2">
        <ChannelTag channel={channel} />
        {channel === "EMAIL" && (
          <div className="overflow-hidden rounded-md bg-white text-[#18181b]">
            <div className="border-b border-[#e4e4e7] px-4 py-2.5 text-sm font-semibold">{subject.trim() || "No subject"}</div>
            {emailHTML.trim() ? (
              <iframe title="Email HTML preview" sandbox="" srcDoc={emailHTML} className="h-[28rem] w-full bg-white" />
            ) : (
              <p className="px-4 py-3 text-sm whitespace-pre-wrap">{shownBody}</p>
            )}
          </div>
        )}
        {channel === "SMS" && (
          <div className="max-w-[85%] rounded-2xl rounded-bl-sm bg-[#26262b] px-3.5 py-2.5 text-sm whitespace-pre-wrap text-[#f4f4f5]">
            {shownBody}
          </div>
        )}
        {channel === "DISCORD" && (
          <div className="overflow-hidden rounded-md bg-[#313338] text-[#dbdee1]">
            {discordChannel && (
              <div className="flex items-center gap-1.5 border-b border-black/20 px-3 py-2 text-sm font-semibold text-[#f2f3f5]">
                <Hash className="size-4 text-[#80848e]" />
                {discordChannel}
              </div>
            )}
            <div className="flex gap-3 p-3">
              <Avatar>
                {botAvatar && <AvatarImage src={botAvatar} alt="" />}
                <AvatarFallback>{botName.slice(0, 1).toUpperCase()}</AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <div className="mb-1 flex items-center gap-1.5">
                  <span className="text-sm font-medium text-[#f2f3f5]">{botName}</span>
                  <span className="rounded-[3px] bg-[#5865f2] px-1 text-[10px] font-semibold text-white">APP</span>
                </div>
                {shownBody && <div className="text-sm whitespace-pre-wrap">{shownBody}</div>}
                {discordEmbed && (
                  <div
                    className="mt-1 max-w-md rounded-sm border-l-4 bg-[#2b2d31] px-3.5 py-3"
                    style={{ borderColor: `#${(discordEmbed.color ?? 0).toString(16).padStart(6, "0")}` }}
                  >
                    {discordEmbed.title && (
                      <div className={cn("mb-1 text-sm font-semibold", discordEmbed.url ? "text-[#00a8fc]" : "text-[#f2f3f5]")}>
                        {discordEmbed.title}
                      </div>
                    )}
                    {discordEmbed.description && <div className="text-sm whitespace-pre-wrap">{discordEmbed.description}</div>}
                    {discordEmbed.footer && <div className="mt-2 text-[11px] text-[#949ba4]">{discordEmbed.footer.text}</div>}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </Panel>
  )
}

function TransmissionLog({ messageID, channelNames, onClear }: { messageID: string; channelNames: Map<string, string>; onClear: () => void }) {
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
  const directory = useIdentityDirectory(deliveries.flatMap((delivery) => (delivery.entity_id ? [delivery.entity_id] : [])))
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
              {delivery.discord_channel_id ? (
                <span className="flex min-w-0 items-center gap-1.5 text-xs font-medium">
                  <Hash className="size-3.5 text-muted-foreground" />
                  <span className="truncate">{channelNames.get(delivery.discord_channel_id) ?? delivery.discord_channel_id}</span>
                </span>
              ) : (
                <IdentityDisplay
                  entityID={delivery.entity_id ?? ""}
                  identity={directory.byID.get(delivery.entity_id ?? "")}
                  loading={directory.isLoading}
                  size="sm"
                  showDetails={false}
                />
              )}
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
