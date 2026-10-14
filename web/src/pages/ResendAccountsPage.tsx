import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ChevronDown, ChevronRight, KeyRound, Pencil, Plus, Trash2 } from "lucide-react"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PageContainer, PageHeader } from "@/components/PageContainer"
import { ChannelTag, ChannelTile } from "@/components/Telemetry"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { apiErrorMessage } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import {
  createResendAccount,
  deleteResendAccount,
  listResendAccountDomains,
  listResendAccounts,
  RESEND_ACCOUNTS_KEY,
  updateResendAccount,
  type ResendAccount,
  type ResendAccountInput,
} from "@/lib/notify"
import { cn } from "@/lib/utils"

export default function ResendAccountsPage() {
  const { isAdmin, isLoading: authLoading } = useAuth()
  const [editing, setEditing] = useState<ResendAccount | "new" | null>(null)
  const { data: accounts, isLoading, error } = useQuery({
    queryKey: RESEND_ACCOUNTS_KEY,
    queryFn: listResendAccounts,
    enabled: isAdmin,
  })

  if (!authLoading && !isAdmin) {
    return (
      <PageContainer>
        <PageHeader title="Resend accounts" />
        <p className="text-sm text-muted-foreground">Only Notify admins can manage Resend accounts.</p>
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <PageHeader
        eyebrow={<ChannelTag channel="EMAIL" />}
        title="Resend accounts"
        description="Resend API keys Notify sends email through. Each email sender belongs to one account."
        action={
          <Button onClick={() => setEditing("new")}>
            <Plus className="size-4" />
            Add account
          </Button>
        }
      />

      {error && (
        <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {apiErrorMessage(error, "Failed to load Resend accounts.")}
        </div>
      )}

      <div className="space-y-3">
        {(isLoading || authLoading) && <Skeleton className="h-24 w-full" />}
        {accounts?.length === 0 && (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
              No Resend accounts yet. Add one before creating email senders.
            </CardContent>
          </Card>
        )}
        {accounts?.map((account) => (
          <ResendAccountRow key={account.id} account={account} onEdit={() => setEditing(account)} />
        ))}
      </div>

      <ResendAccountDialog
        key={editing === "new" ? "new" : editing?.id ?? "closed"}
        account={editing === "new" ? null : editing}
        open={editing !== null}
        onClose={() => setEditing(null)}
      />
    </PageContainer>
  )
}

function ResendAccountRow({ account, onEdit }: { account: ResendAccount; onEdit: () => void }) {
  const queryClient = useQueryClient()
  const [showDomains, setShowDomains] = useState(false)
  const invalidate = () => queryClient.invalidateQueries({ queryKey: RESEND_ACCOUNTS_KEY })

  const update = useMutation({
    mutationFn: (input: ResendAccountInput) => updateResendAccount(account.id, input),
    onSuccess: invalidate,
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to update account.")),
  })
  const remove = useMutation({
    mutationFn: () => deleteResendAccount(account.id),
    onSuccess: () => {
      toast.success(`Deleted ${account.name}`)
      return invalidate()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to delete account.")),
  })

  return (
    <Card className={cn(!account.enabled && "opacity-60")}>
      <CardContent className="space-y-3">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <ChannelTile channel="EMAIL">
              <KeyRound className="size-4" />
            </ChannelTile>
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium">{account.name}</span>
                <Badge variant="outline">{account.full_access ? "Full access" : "Sending only"}</Badge>
                {!account.enabled && <Badge variant="destructive">Disabled</Badge>}
              </div>
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                <span className="font-mono">key ••••{account.key_hint}</span>
                <span>
                  {account.sender_count} {account.sender_count === 1 ? "sender" : "senders"}
                </span>
              </div>
              {account.description && <p className="text-sm text-muted-foreground">{account.description}</p>}
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {account.full_access && (
              <Button variant="ghost" size="sm" onClick={() => setShowDomains((value) => !value)}>
                {showDomains ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                Domains
              </Button>
            )}
            <Button variant="outline" size="sm" disabled={update.isPending} onClick={() => update.mutate({ enabled: !account.enabled })}>
              {account.enabled ? "Disable" : "Enable"}
            </Button>
            <Button variant="outline" size="sm" onClick={onEdit}>
              <Pencil className="size-3.5" />
              Edit
            </Button>
            <ConfirmDialog
              trigger={
                <Button variant="destructive" size="sm" disabled={account.sender_count > 0}>
                  <Trash2 className="size-3.5" />
                  Delete
                </Button>
              }
              title={`Delete ${account.name}?`}
              description="The API key is removed from Notify. It stays active in Resend until you revoke it there."
              confirmLabel="Delete"
              isPending={remove.isPending}
              onConfirm={async () => {
                await remove.mutateAsync()
              }}
            />
          </div>
        </div>
        {showDomains && <ResendDomains accountID={account.id} />}
      </CardContent>
    </Card>
  )
}

function ResendDomains({ accountID }: { accountID: string }) {
  const { data: domains, isLoading, error } = useQuery({
    queryKey: [...RESEND_ACCOUNTS_KEY, accountID, "domains"],
    queryFn: () => listResendAccountDomains(accountID),
  })

  if (isLoading) return <Skeleton className="h-8 w-full" />
  if (error) return <p className="text-sm text-destructive">{apiErrorMessage(error, "Failed to load domains.")}</p>
  if (!domains?.length) return <p className="text-sm text-muted-foreground">No domains on this account.</p>

  return (
    <div className="flex flex-wrap gap-2 border-t border-border/60 pt-3">
      {domains.map((domain) => (
        <Badge
          key={domain.name}
          variant={domain.status === "verified" || domain.status === "partially_verified" ? "secondary" : "outline"}
          className="font-mono"
        >
          {domain.name} · {domain.status.replaceAll("_", " ")}
        </Badge>
      ))}
    </div>
  )
}

function ResendAccountDialog({
  account,
  open,
  onClose,
}: {
  account: ResendAccount | null
  open: boolean
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(account?.name ?? "")
  const [description, setDescription] = useState(account?.description ?? "")
  const [apiKey, setAPIKey] = useState("")

  const save = useMutation({
    mutationFn: () => {
      const input: ResendAccountInput = { name, description }
      if (apiKey.trim()) input.api_key = apiKey.trim()
      return account ? updateResendAccount(account.id, input) : createResendAccount(input)
    },
    onSuccess: async (saved) => {
      toast.success(account ? `Updated ${saved.name}` : `Added ${saved.name}`)
      await queryClient.invalidateQueries({ queryKey: RESEND_ACCOUNTS_KEY })
      onClose()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to save account.")),
  })

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    save.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{account ? `Edit ${account.name}` : "Add Resend account"}</DialogTitle>
            <DialogDescription>
              The key is checked with Resend, then stored encrypted. It is never shown again. A full-access key lets
              Notify check sender domains before saving; a sending-only key works but skips that check.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="account-name">Name</Label>
            <Input id="account-name" value={name} onChange={(event) => setName(event.target.value)} maxLength={64} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="account-key">API key</Label>
            <Input
              id="account-key"
              type="password"
              autoComplete="off"
              value={apiKey}
              onChange={(event) => setAPIKey(event.target.value)}
              placeholder={account ? `Leave blank to keep ••••${account.key_hint}` : "re_…"}
              required={!account}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="account-description">Description</Label>
            <Textarea
              id="account-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              maxLength={500}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? "Saving" : account ? "Save" : "Add account"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
