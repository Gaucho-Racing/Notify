import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"

import { api } from "@/lib/api"
import type { SentinelApplication } from "@/lib/notify"

export type IdentityApplicationSummary = {
  id: string
  name: string
  client_id: string
  icon_url: string
}

export type IdentitySummary = {
  id: string
  type: "USER" | "SERVICE_ACCOUNT"
  name: string
  username?: string
  avatar_url?: string
  application?: IdentityApplicationSummary
}

function normalized(values: string[]) {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))].sort()
}

async function resolveIdentities(ids: string[]) {
  const response = await api.post<IdentitySummary[]>("/identities/resolve", { ids })
  return response.data
}

async function resolveApplications(clientIDs: string[]) {
  const response = await api.post<SentinelApplication[]>("/applications/resolve", {
    client_ids: clientIDs,
  })
  return response.data
}

export function useIdentityDirectory(entityIDs: string[]) {
  const ids = normalized(entityIDs)
  const query = useQuery({
    queryKey: ["identityDirectory", ids],
    queryFn: () => resolveIdentities(ids),
    enabled: ids.length > 0,
    staleTime: 5 * 60 * 1000,
  })
  const byID = useMemo(
    () => new Map((query.data ?? []).map((identity) => [identity.id, identity])),
    [query.data],
  )
  return { ...query, byID }
}

export function useApplicationDirectory(clientIDs: string[]) {
  const ids = normalized(clientIDs)
  const query = useQuery({
    queryKey: ["applicationDirectory", ids],
    queryFn: () => resolveApplications(ids),
    enabled: ids.length > 0,
    staleTime: 5 * 60 * 1000,
  })
  const byClientID = useMemo(
    () => new Map((query.data ?? []).map((application) => [application.client_id, application])),
    [query.data],
  )
  return { ...query, byClientID }
}
