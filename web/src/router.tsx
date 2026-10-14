import { createBrowserRouter, Navigate } from "react-router-dom"

import { AppShell } from "@/components/AppShell"
import { RequireAuth } from "@/components/RequireAuth"
import DashboardPage from "@/pages/DashboardPage"
import DiscordBotsPage from "@/pages/DiscordBotsPage"
import EmailSendersPage from "@/pages/EmailSendersPage"
import ResendAccountsPage from "@/pages/ResendAccountsPage"
import LoginPage from "@/pages/LoginPage"
import NotFoundPage from "@/pages/NotFoundPage"
import PlaygroundPage from "@/pages/PlaygroundPage"
import SettingsPage from "@/pages/SettingsPage"
import SMSSendersPage from "@/pages/SMSSendersPage"
import TwilioAccountsPage from "@/pages/TwilioAccountsPage"

export const router = createBrowserRouter([
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppShell />,
        children: [
          { path: "/", element: <Navigate to="/dashboard" replace /> },
          { path: "/dashboard", element: <DashboardPage /> },
          { path: "/playground", element: <PlaygroundPage /> },
          { path: "/email-senders", element: <EmailSendersPage /> },
          { path: "/resend-accounts", element: <ResendAccountsPage /> },
          { path: "/sms-senders", element: <SMSSendersPage /> },
          { path: "/twilio-accounts", element: <TwilioAccountsPage /> },
          { path: "/discord-bots", element: <DiscordBotsPage /> },
          { path: "/settings", element: <SettingsPage /> },
        ],
      },
    ],
  },
  { path: "/auth/login", element: <LoginPage /> },
  { path: "*", element: <NotFoundPage /> },
])
