import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ChevronDown, ChevronRight, Pencil, Phone, Plus, Trash2 } from "lucide-react"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PageContainer, PageHeader } from "@/components/PageContainer"
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
  createTwilioAccount,
  deleteTwilioAccount,
  listTwilioAccounts,
  listTwilioSenderOptions,
  TWILIO_ACCOUNTS_KEY,
  updateTwilioAccount,
  type TwilioAccount,
  type TwilioAccountInput,
} from "@/lib/notify"

export default function TwilioAccountsPage() {
  const { isAdmin, isLoading: authLoading } = useAuth()
  const [editing, setEditing] = useState<TwilioAccount | "new" | null>(null)
  const { data: accounts, isLoading, error } = useQuery({
    queryKey: TWILIO_ACCOUNTS_KEY,
    queryFn: listTwilioAccounts,
    enabled: isAdmin,
  })

  if (!authLoading && !isAdmin) {
    return (
      <PageContainer>
        <PageHeader title="Twilio accounts" />
        <p className="text-sm text-muted-foreground">Only Notify admins can manage Twilio accounts.</p>
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <PageHeader
        title="Twilio accounts"
        description="Twilio credentials Notify sends SMS through. Each SMS sender belongs to one account."
        action={
          <Button onClick={() => setEditing("new")}>
            <Plus className="size-4" />
            Add account
          </Button>
        }
      />

      {error && (
        <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {apiErrorMessage(error, "Failed to load Twilio accounts.")}
        </div>
      )}

      <div className="space-y-3">
        {(isLoading || authLoading) && <Skeleton className="h-24 w-full" />}
        {accounts?.length === 0 && (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
              No Twilio accounts yet. Add one before creating SMS senders.
            </CardContent>
          </Card>
        )}
        {accounts?.map((account) => (
          <TwilioAccountRow key={account.id} account={account} onEdit={() => setEditing(account)} />
        ))}
      </div>

      <TwilioAccountDialog
        key={editing === "new" ? "new" : editing?.id ?? "closed"}
        account={editing === "new" ? null : editing}
        open={editing !== null}
        onClose={() => setEditing(null)}
      />
    </PageContainer>
  )
}

function TwilioAccountRow({ account, onEdit }: { account: TwilioAccount; onEdit: () => void }) {
  const queryClient = useQueryClient()
  const [showSenders, setShowSenders] = useState(false)
  const invalidate = () => queryClient.invalidateQueries({ queryKey: TWILIO_ACCOUNTS_KEY })

  const update = useMutation({
    mutationFn: (input: TwilioAccountInput) => updateTwilioAccount(account.id, input),
    onSuccess: invalidate,
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to update account.")),
  })
  const remove = useMutation({
    mutationFn: () => deleteTwilioAccount(account.id),
    onSuccess: () => {
      toast.success(`Deleted ${account.name}`)
      return invalidate()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to delete account.")),
  })

  return (
    <Card>
      <CardContent className="space-y-3">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
              <Phone className="size-5" />
            </div>
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium">{account.name}</span>
                <Badge variant="outline">{account.key_sid ? "API key" : "Auth token"}</Badge>
                {!account.enabled && <Badge variant="destructive">Disabled</Badge>}
              </div>
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                <span className="break-all font-mono">{account.account_sid}</span>
                {account.key_sid && <span className="break-all font-mono">{account.key_sid}</span>}
                <span className="font-mono">secret ••••{account.secret_hint}</span>
                <span>
                  {account.sender_count} {account.sender_count === 1 ? "sender" : "senders"}
                </span>
              </div>
              {account.description && <p className="text-sm text-muted-foreground">{account.description}</p>}
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button variant="ghost" size="sm" onClick={() => setShowSenders((value) => !value)}>
              {showSenders ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
              Numbers
            </Button>
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
              description="The credentials are removed from Notify. They stay active in Twilio until you revoke them there."
              confirmLabel="Delete"
              isPending={remove.isPending}
              onConfirm={async () => {
                await remove.mutateAsync()
              }}
            />
          </div>
        </div>
        {showSenders && <TwilioSenderOptionsList accountID={account.id} />}
      </CardContent>
    </Card>
  )
}

function TwilioSenderOptionsList({ accountID }: { accountID: string }) {
  const { data, isLoading, error } = useQuery({
    queryKey: [...TWILIO_ACCOUNTS_KEY, accountID, "senders"],
    queryFn: () => listTwilioSenderOptions(accountID),
  })

  if (isLoading) return <Skeleton className="h-8 w-full" />
  if (error) return <p className="text-sm text-destructive">{apiErrorMessage(error, "Failed to load numbers.")}</p>
  if (!data?.phone_numbers.length && !data?.messaging_services.length) {
    return <p className="text-sm text-muted-foreground">No phone numbers or messaging services on this account.</p>
  }

  return (
    <div className="flex flex-wrap gap-2 border-t border-border/60 pt-3">
      {data.phone_numbers.map((number) => (
        <Badge key={number.phone_number} variant={number.sms ? "secondary" : "outline"} className="font-mono">
          {number.phone_number}
          {!number.sms && " · no SMS"}
        </Badge>
      ))}
      {data.messaging_services.map((service) => (
        <Badge key={service.sid} variant="secondary">
          {service.friendly_name} · messaging service
        </Badge>
      ))}
    </div>
  )
}

function TwilioAccountDialog({
  account,
  open,
  onClose,
}: {
  account: TwilioAccount | null
  open: boolean
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(account?.name ?? "")
  const [description, setDescription] = useState(account?.description ?? "")
  const [accountSID, setAccountSID] = useState(account?.account_sid ?? "")
  const [keySID, setKeySID] = useState(account?.key_sid ?? "")
  const [secret, setSecret] = useState("")
  const keyChanged = account !== null && keySID.trim() !== account.key_sid

  const save = useMutation({
    mutationFn: () => {
      const input: TwilioAccountInput = { name, description }
      if (!account) input.account_sid = accountSID.trim()
      if (secret.trim()) {
        input.secret = secret.trim()
        input.key_sid = keySID.trim()
      }
      return account ? updateTwilioAccount(account.id, input) : createTwilioAccount(input)
    },
    onSuccess: async (saved) => {
      toast.success(account ? `Updated ${saved.name}` : `Added ${saved.name}`)
      await queryClient.invalidateQueries({ queryKey: TWILIO_ACCOUNTS_KEY })
      onClose()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to save account.")),
  })

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (keyChanged && !secret.trim()) {
      toast.error("Enter the secret for the new API key.")
      return
    }
    save.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{account ? `Edit ${account.name}` : "Add Twilio account"}</DialogTitle>
            <DialogDescription>
              Use an API key (recommended) or leave the key SID blank to use the account's auth token. The credentials are
              checked with Twilio, then the secret is stored encrypted and never shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="twilio-name">Name</Label>
            <Input id="twilio-name" value={name} onChange={(event) => setName(event.target.value)} maxLength={64} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="twilio-account-sid">Account SID</Label>
            <Input
              id="twilio-account-sid"
              value={accountSID}
              onChange={(event) => setAccountSID(event.target.value)}
              placeholder="AC…"
              className="font-mono"
              disabled={account !== null}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="twilio-key-sid">API key SID</Label>
            <Input
              id="twilio-key-sid"
              value={keySID}
              onChange={(event) => setKeySID(event.target.value)}
              placeholder="SK… (blank for auth token)"
              className="font-mono"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="twilio-secret">{keySID.trim() ? "API key secret" : "Auth token"}</Label>
            <Input
              id="twilio-secret"
              type="password"
              autoComplete="off"
              value={secret}
              onChange={(event) => setSecret(event.target.value)}
              placeholder={account && !keyChanged ? `Leave blank to keep ••••${account.secret_hint}` : ""}
              required={!account || keyChanged}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="twilio-description">Description</Label>
            <Textarea
              id="twilio-description"
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
