import { useEffect, useState } from "react"

export type ResolvedTheme = "light" | "dark"
export type Theme = ResolvedTheme | "system"

const STORAGE_KEY = "notify_theme"
const EVENT_NAME = "notify-theme-change"
const DARK_MODE_QUERY = "(prefers-color-scheme: dark)"

function systemTheme(): ResolvedTheme {
  return window.matchMedia(DARK_MODE_QUERY).matches ? "dark" : "light"
}

export function currentTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === "light" || stored === "dark" || stored === "system") return stored
  return "system"
}

function resolveTheme(theme: Theme): ResolvedTheme {
  return theme === "system" ? systemTheme() : theme
}

function applyTheme(theme: Theme): ResolvedTheme {
  const resolved = resolveTheme(theme)
  document.documentElement.classList.toggle("dark", resolved === "dark")
  document.documentElement.style.colorScheme = resolved
  return resolved
}

export function setTheme(theme: Theme) {
  localStorage.setItem(STORAGE_KEY, theme)
  applyTheme(theme)
  window.dispatchEvent(new CustomEvent(EVENT_NAME, { detail: theme }))
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(() => currentTheme())
  const [resolved, setResolved] = useState<ResolvedTheme>(() => resolveTheme(currentTheme()))

  useEffect(() => {
    function syncTheme() {
      const nextTheme = currentTheme()
      setThemeState(nextTheme)
      setResolved(applyTheme(nextTheme))
    }

    function handleSystemThemeChange() {
      if (currentTheme() === "system") {
        syncTheme()
      }
    }

    const media = window.matchMedia(DARK_MODE_QUERY)
    syncTheme()
    window.addEventListener(EVENT_NAME, syncTheme)
    window.addEventListener("storage", syncTheme)
    media.addEventListener("change", handleSystemThemeChange)
    return () => {
      window.removeEventListener(EVENT_NAME, syncTheme)
      window.removeEventListener("storage", syncTheme)
      media.removeEventListener("change", handleSystemThemeChange)
    }
  }, [])

  function updateTheme(nextTheme: Theme) {
    setThemeState(nextTheme)
    setResolved(applyTheme(nextTheme))
    setTheme(nextTheme)
  }

  function toggleTheme() {
    updateTheme(resolved === "dark" ? "light" : "dark")
  }

  return { theme, resolvedTheme: resolved, setTheme: updateTheme, toggleTheme }
}
