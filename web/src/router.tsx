import { createBrowserRouter, Navigate } from "react-router-dom"

import { AppShell } from "@/components/AppShell"
import { RequireAuth } from "@/components/RequireAuth"
import DashboardPage from "@/pages/DashboardPage"
import DiscordBotsPage from "@/pages/DiscordBotsPage"
import LoginPage from "@/pages/LoginPage"
import NotFoundPage from "@/pages/NotFoundPage"
import SettingsPage from "@/pages/SettingsPage"

export const router = createBrowserRouter([
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppShell />,
        children: [
          { path: "/", element: <Navigate to="/dashboard" replace /> },
          { path: "/dashboard", element: <DashboardPage /> },
          { path: "/discord-bots", element: <DiscordBotsPage /> },
          { path: "/settings", element: <SettingsPage /> },
        ],
      },
    ],
  },
  { path: "/auth/login", element: <LoginPage /> },
  { path: "*", element: <NotFoundPage /> },
])
