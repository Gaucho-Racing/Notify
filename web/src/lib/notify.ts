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
