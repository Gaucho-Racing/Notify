import { useQuery } from "@tanstack/react-query"

import { api } from "@/lib/api"

export type NotifyStatus = { version: string | null; env: string | null }

export function useNotifyStatus(): NotifyStatus {
  const { data } = useQuery({
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

  return data ?? { version: null, env: null }
}
