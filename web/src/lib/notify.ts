import { api } from "@/lib/api"

export type SentinelApplication = {
  id: string
  name: string
  description: string
  client_id: string
  icon_url: string
}

export type Channel = "EMAIL" | "DISCORD" | "SMS"

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

export type ActivityPoint = StatusCounts & { date: string }

export async function getActivityStats(days: number) {
  const response = await api.get<ActivityPoint[]>("/stats/activity", { params: { days } })
  return response.data
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
  resend_account_id: string
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
  resend_account_id: string
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

export const RESEND_ACCOUNTS_KEY = ["resendAccounts"]

export type ResendAccount = {
  id: string
  name: string
  description: string
  key_hint: string
  full_access: boolean
  enabled: boolean
  sender_count: number
  created_at: string
  updated_at: string
}

export type ResendAccountInput = Partial<{
  name: string
  description: string
  api_key: string
  enabled: boolean
}>

export type ResendDomain = {
  name: string
  status: string
}

export async function listResendAccounts() {
  const response = await api.get<ResendAccount[]>("/resend-accounts")
  return response.data
}

export async function listResendAccountDomains(id: string) {
  const response = await api.get<ResendDomain[]>(`/resend-accounts/${encodeURIComponent(id)}/domains`)
  return response.data
}

export async function createResendAccount(input: ResendAccountInput) {
  const response = await api.post<ResendAccount>("/resend-accounts", input)
  return response.data
}

export async function updateResendAccount(id: string, input: ResendAccountInput) {
  const response = await api.patch<ResendAccount>(`/resend-accounts/${encodeURIComponent(id)}`, input)
  return response.data
}

export async function deleteResendAccount(id: string) {
  await api.delete(`/resend-accounts/${encodeURIComponent(id)}`)
}

export const TWILIO_ACCOUNTS_KEY = ["twilioAccounts"]

export type TwilioAccount = {
  id: string
  name: string
  description: string
  account_sid: string
  key_sid: string
  secret_hint: string
  enabled: boolean
  sender_count: number
  created_at: string
  updated_at: string
}

export type TwilioAccountInput = Partial<{
  name: string
  description: string
  account_sid: string
  key_sid: string
  secret: string
  enabled: boolean
}>

export type TwilioSenderOptions = {
  phone_numbers: { phone_number: string; friendly_name: string; sms: boolean }[]
  messaging_services: { sid: string; friendly_name: string }[]
}

export async function listTwilioAccounts() {
  const response = await api.get<TwilioAccount[]>("/twilio-accounts")
  return response.data
}

export async function listTwilioSenderOptions(id: string) {
  const response = await api.get<TwilioSenderOptions>(`/twilio-accounts/${encodeURIComponent(id)}/senders`)
  return response.data
}

export async function createTwilioAccount(input: TwilioAccountInput) {
  const response = await api.post<TwilioAccount>("/twilio-accounts", input)
  return response.data
}

export async function updateTwilioAccount(id: string, input: TwilioAccountInput) {
  const response = await api.patch<TwilioAccount>(`/twilio-accounts/${encodeURIComponent(id)}`, input)
  return response.data
}

export async function deleteTwilioAccount(id: string) {
  await api.delete(`/twilio-accounts/${encodeURIComponent(id)}`)
}

export type SMSSender = {
  id: string
  name: string
  description: string
  twilio_account_id: string
  from_number: string
  messaging_service_sid: string
  is_default: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export type SMSSenderInput = Partial<{
  name: string
  description: string
  twilio_account_id: string
  from_number: string
  messaging_service_sid: string
  is_default: boolean
  enabled: boolean
}>

export async function listSMSSenders() {
  const response = await api.get<SMSSender[]>("/sms-senders")
  return response.data
}

export async function createSMSSender(input: SMSSenderInput) {
  const response = await api.post<SMSSender>("/sms-senders", input)
  return response.data
}

export async function updateSMSSender(id: string, input: SMSSenderInput) {
  const response = await api.patch<SMSSender>(`/sms-senders/${encodeURIComponent(id)}`, input)
  return response.data
}

export async function deleteSMSSender(id: string) {
  await api.delete(`/sms-senders/${encodeURIComponent(id)}`)
}

export type SentinelGroup = {
  id: string
  name: string
  description: string
  member_count: number
}

export type SentinelUser = {
  entity_id: string
  name: string
  username: string
  avatar_url: string
}

export async function listGroups() {
  const response = await api.get<SentinelGroup[]>("/groups")
  return response.data
}

export async function listUsers() {
  const response = await api.get<SentinelUser[]>("/users")
  return response.data
}

export type DeliveryStatus = "PENDING" | "SENDING" | "SENT" | "FAILED" | "SKIPPED"

export type Delivery = {
  id: string
  message_id: string
  entity_id: string
  channel: Channel
  provider: string
  destination: string
  status: DeliveryStatus
  attempts: number
  last_error?: string
  sent_at?: string
  created_at: string
}

export type SendMessageInput = {
  recipients: { entity_ids: string[]; group_ids: string[] }
  channels: Channel[]
  email_sender_id?: string
  sms_sender_id?: string
  discord_bot_id?: string
  subject: string
  body: string
  action_url?: string
  action_label?: string
  category?: string
}

export type Message = {
  id: string
  subject: string
  channels: Channel[]
  recipient_count: number
  created_at: string
}

export type MessageDetail = {
  message: Message
  summary: Partial<Record<DeliveryStatus, number>>
  deliveries: Delivery[]
}

export async function sendMessage(input: SendMessageInput) {
  const response = await api.post<Message>("/messages", input)
  return response.data
}

export async function getMessage(id: string) {
  const response = await api.get<MessageDetail>(`/messages/${encodeURIComponent(id)}`)
  return response.data
}
