import { useQuery } from "@tanstack/react-query"

import { listSentinelApplications } from "@/lib/notify"

/**
 * useApplications loads the Sentinel application list that Notify proxies.
 * Shared by every policy surface so the list is fetched once per session.
 */
export function useApplications() {
  return useQuery({
    queryKey: ["sentinelApplications"],
    queryFn: listSentinelApplications,
    staleTime: 5 * 60 * 1000,
  })
}
