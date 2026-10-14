import { api } from "@/lib/api"

export type SentinelApplication = {
  id: string
  name: string
  description: string
  client_id: string
  icon_url: string
}

export type Channel = "EMAIL" | "DISCORD"

export type StatusCounts = {
  pending: number
  sent: number
  failed: number
  skipped: number
}

export type ApplicationStats = {
  client_id: string
  messages: number
  deliveries: StatusCounts
}

export type ErrorCount = {
  channel: Channel
  error: string
  count: number
}

export type Stats = {
  since: string
  messages: number
  deliveries: StatusCounts
  by_channel: Partial<Record<Channel, StatusCounts>>
  applications: ApplicationStats[]
  top_errors: ErrorCount[]
}

export async function listSentinelApplications() {
  const response = await api.get<SentinelApplication[]>("/applications")
  return response.data
}

export async function getStats(days: number) {
  const response = await api.get<Stats>("/stats", { params: { days } })
  return response.data
}

export type DiscordBot = {
  id: string
  name: string
  description: string
  discord_user_id: string
  discord_username: string
  avatar_url: string
  token_hint: string
  is_default: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export type DiscordBotInput = Partial<{
  name: string
  description: string
  token: string
  is_default: boolean
  enabled: boolean
}>

export async function listDiscordBots() {
  const response = await api.get<DiscordBot[]>("/discord-bots")
  return response.data
}

export async function createDiscordBot(input: DiscordBotInput) {
  const response = await api.post<DiscordBot>("/discord-bots", input)
  return response.data
}

export async function updateDiscordBot(id: string, input: DiscordBotInput) {
  const response = await api.patch<DiscordBot>(`/discord-bots/${encodeURIComponent(id)}`, input)
  return response.data
}

export async function deleteDiscordBot(id: string) {
  await api.delete(`/discord-bots/${encodeURIComponent(id)}`)
}

export type EmailSender = {
  id: string
  name: string
  description: string
  from_name: string
  from_address: string
  reply_to: string
  is_default: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export type EmailSenderInput = Partial<{
  name: string
  description: string
  from_name: string
  from_address: string
  reply_to: string
  is_default: boolean
  enabled: boolean
}>

export async function listEmailSenders() {
  const response = await api.get<EmailSender[]>("/email-senders")
  return response.data
}

export async function createEmailSender(input: EmailSenderInput) {
  const response = await api.post<EmailSender>("/email-senders", input)
  return response.data
}

export async function updateEmailSender(id: string, input: EmailSenderInput) {
  const response = await api.patch<EmailSender>(`/email-senders/${encodeURIComponent(id)}`, input)
  return response.data
}

export async function deleteEmailSender(id: string) {
  await api.delete(`/email-senders/${encodeURIComponent(id)}`)
}

export function formatEmailFrom(sender: Pick<EmailSender, "from_name" | "from_address">) {
  return sender.from_name ? `${sender.from_name} <${sender.from_address}>` : sender.from_address
}
