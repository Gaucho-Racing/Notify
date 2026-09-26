import { Moon, Sun } from "lucide-react"

import { PageContainer, PageHeader } from "@/components/PageContainer"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useAuth } from "@/lib/auth"
import { useTheme } from "@/lib/theme"

export default function SettingsPage() {
  const { resolvedTheme, toggleTheme } = useTheme()
  const { user } = useAuth()

  return (
    <PageContainer>
      <PageHeader title="Settings" description="Appearance and account details." />

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Appearance</CardTitle>
            <CardDescription>Switch between light and dark mode.</CardDescription>
          </CardHeader>
          <CardContent>
            <Button variant="outline" onClick={toggleTheme}>
              {resolvedTheme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
              {resolvedTheme === "dark" ? "Light mode" : "Dark mode"}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Account</CardTitle>
            <CardDescription>Signed in through Sentinel.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <p>
              <span className="text-muted-foreground">Name: </span>
              {user ? `${user.first_name} ${user.last_name}`.trim() : "—"}
            </p>
            <p>
              <span className="text-muted-foreground">Email: </span>
              {user?.email ?? "—"}
            </p>
            <p className="flex flex-wrap gap-1.5">
              <span className="text-muted-foreground">Groups: </span>
              {user?.groups?.length ? user.groups.join(", ") : "none"}
            </p>
          </CardContent>
        </Card>
      </div>
    </PageContainer>
  )
}
