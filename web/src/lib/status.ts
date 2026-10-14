import { useQuery } from "@tanstack/react-query"

import { api } from "@/lib/api"

export type NotifyStatus = { version: string | null; env: string | null; online: boolean | null }

export function useNotifyStatus(): NotifyStatus {
  const { data, isError } = useQuery({
    queryKey: ["notify-status"],
    queryFn: async () => {
      const response = await api.get<{ message?: string; env?: string }>("/ping")
      const match = response.data?.message?.match(/v([\d.]+)/)
      return {
        version: match ? `v${match[1]}` : null,
        env: response.data?.env ?? null,
      }
    },
    staleTime: 60_000,
    refetchInterval: 60_000,
    retry: 2,
  })

  if (data) return { ...data, online: !isError }
  return { version: null, env: null, online: isError ? false : null }
}

export function envLabel(env: string | null) {
  if (!env) return null
  return env.toUpperCase() === "PROD" ? "Production" : "Development"
}
