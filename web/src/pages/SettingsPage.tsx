import { PageContainer, PageHeader } from "@/components/PageContainer"
import { Eyebrow } from "@/components/Telemetry"
import { Badge } from "@/components/ui/badge"
import { useAuth } from "@/lib/auth"

export default function SettingsPage() {
  const { user, isAdmin } = useAuth()
  const name = user ? `${user.first_name} ${user.last_name}`.trim() || user.username : null

  const rows = [
    { label: "Name", value: name ?? "—" },
    { label: "Email", value: user?.email ?? "—" },
    { label: "Role", value: isAdmin ? "Notify admin" : "Member" },
  ]

  return (
    <PageContainer>
      <PageHeader eyebrow="System" title="Settings" description="Your account, as Sentinel reports it." />

      <section className="max-w-2xl overflow-hidden rounded-lg border border-border bg-card/90">
        <div className="border-b border-border px-5 py-3">
          <Eyebrow>Account · signed in through Sentinel</Eyebrow>
        </div>
        <dl className="divide-y divide-border text-sm">
          {rows.map((row) => (
            <div key={row.label} className="grid grid-cols-[96px_minmax(0,1fr)] gap-4 px-5 py-3">
              <dt className="font-mono text-[11px] uppercase tracking-[0.12em] text-muted-foreground">{row.label}</dt>
              <dd className="break-words">{row.value}</dd>
            </div>
          ))}
          <div className="grid grid-cols-[96px_minmax(0,1fr)] gap-4 px-5 py-3">
            <dt className="font-mono text-[11px] uppercase tracking-[0.12em] text-muted-foreground">Groups</dt>
            <dd className="flex flex-wrap gap-1.5">
              {user?.groups?.length ? (
                user.groups.map((group) => (
                  <Badge key={group} variant={group === "NotifyAdmins" ? "default" : "outline"} className="normal-case tracking-normal">
                    {group}
                  </Badge>
                ))
              ) : (
                <span className="text-muted-foreground">none</span>
              )}
            </dd>
          </div>
        </dl>
      </section>
    </PageContainer>
  )
}
