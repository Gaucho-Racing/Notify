import { useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "@/lib/api"

const SESSION_KEY = "notify_session"

export type Session = {
  accessToken: string
  refreshToken: string
  expiresIn: number
}

export type CurrentUser = {
  id: string
  entity_id: string
  username: string
  first_name: string
  last_name: string
  email: string
  phone_number: string
  gender: string
  birthday: string
  graduate_level: string
  graduation_year: number
  major: string
  shirt_size: string
  jacket_size: string
  sae_registration_number: string
  occupation_title: string
  occupation_company: string
  avatar_url: string
  initial_role: string
  groups: string[]
  updated_at: string
  created_at: string
}

export function saveSession(session: Session) {
  localStorage.setItem(SESSION_KEY, JSON.stringify(session))
}

export function loadSession(): Session | null {
  const raw = localStorage.getItem(SESSION_KEY)
  if (!raw) return null
  try {
    const session = JSON.parse(raw) as Partial<Session>
    if (!session.accessToken || !session.refreshToken) {
      localStorage.removeItem(SESSION_KEY)
      return null
    }
    return {
      accessToken: session.accessToken,
      refreshToken: session.refreshToken,
      expiresIn: Number(session.expiresIn) || 0,
    }
  } catch {
    localStorage.removeItem(SESSION_KEY)
    return null
  }
}

export function clearSession() {
  localStorage.removeItem(SESSION_KEY)
}

export type SessionInfo = {
  entity_id: string
  user_id: string
  client_id: string
  scope: string
  groups: string[]
  first_party: boolean
  is_admin: boolean
}

export function useAuth() {
  const queryClient = useQueryClient()
  const tokenSession = loadSession()
  // Notify resolves is_admin itself, so the web app never has to know which
  // Sentinel group the server is configured to trust.
  const sessionQuery = useQuery({
    queryKey: ["session", tokenSession?.accessToken],
    queryFn: async () => {
      const response = await api.get<SessionInfo>("/auth/session")
      return response.data
    },
    enabled: !!tokenSession?.accessToken,
    retry: false,
    staleTime: 5 * 60 * 1000,
  })
  const userQuery = useQuery({
    queryKey: ["currentUser", tokenSession?.accessToken],
    queryFn: async () => {
      const response = await api.get<CurrentUser>("/users/@me")
      return response.data
    },
    enabled: !!tokenSession?.accessToken,
    retry: false,
    staleTime: 5 * 60 * 1000,
  })

  function logout() {
    clearSession()
    queryClient.clear()
    window.location.href = "/auth/login"
  }

  return {
    user: userQuery.data,
    session: sessionQuery.data,
    isAdmin: sessionQuery.data?.is_admin ?? false,
    tokenSession,
    isLoading: userQuery.isLoading || sessionQuery.isLoading,
    isAuthenticated: !!tokenSession,
    refresh: () => queryClient.invalidateQueries({ queryKey: ["currentUser"] }),
    logout,
  }
}
