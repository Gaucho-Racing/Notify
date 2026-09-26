import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios"

import { clearSession, loadSession, saveSession } from "@/lib/auth"

export const apiBaseURL = `${import.meta.env.VITE_API_URL ?? ""}/api`

export const api = axios.create({
  baseURL: apiBaseURL,
  withCredentials: false,
})

type RetriedConfig = InternalAxiosRequestConfig & { _retried?: boolean }

type TokenResponse = {
  access_token: string
  refresh_token: string
  expires_in: number
}

api.interceptors.request.use((config) => {
  const session = loadSession()
  if (session?.accessToken) {
    config.headers.Authorization = `Bearer ${session.accessToken}`
  }
  return config
})

let refreshing: Promise<string | null> | null = null

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as RetriedConfig | undefined
    const status = error.response?.status
    const url = original?.url ?? ""
    const isAuthEndpoint = url.includes("/auth/login") || url.includes("/auth/refresh")

    if (status !== 401 || !original || original._retried || isAuthEndpoint) {
      return Promise.reject(error)
    }
    original._retried = true

    const session = loadSession()
    if (!session?.refreshToken) {
      clearSession()
      window.location.href = "/auth/login"
      return Promise.reject(error)
    }

    refreshing ??= (async () => {
      try {
        const response = await api.post<TokenResponse>("/auth/refresh", {
          refresh_token: session.refreshToken,
        })
        saveSession({
          accessToken: response.data.access_token,
          refreshToken: response.data.refresh_token || session.refreshToken,
          expiresIn: response.data.expires_in,
        })
        return response.data.access_token
      } catch {
        return null
      } finally {
        refreshing = null
      }
    })()

    const accessToken = await refreshing
    if (!accessToken) {
      clearSession()
      window.location.href = "/auth/login"
      return Promise.reject(error)
    }

    original.headers.Authorization = `Bearer ${accessToken}`
    return api(original)
  },
)
