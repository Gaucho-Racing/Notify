import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, CornerUpLeft, KeyRound, Mail, Pencil, Plus, Star, Trash2 } from "lucide-react"
import { useState, type FormEvent } from "react"
import { Link } from "react-router-dom"
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { apiErrorMessage } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import {
  createEmailSender,
  deleteEmailSender,
  formatEmailFrom,
  listEmailSenders,
  listResendAccounts,
  RESEND_ACCOUNTS_KEY,
  updateEmailSender,
  type EmailSender,
  type EmailSenderInput,
  type ResendAccount,
} from "@/lib/notify"

const EMAIL_SENDERS_KEY = ["emailSenders"]

export default function EmailSendersPage() {
  const { isAdmin, isLoading: authLoading } = useAuth()
  const [editing, setEditing] = useState<EmailSender | "new" | null>(null)
  const { data: senders, isLoading, error } = useQuery({
    queryKey: EMAIL_SENDERS_KEY,
    queryFn: listEmailSenders,
    enabled: isAdmin,
  })
  const { data: accounts } = useQuery({
    queryKey: RESEND_ACCOUNTS_KEY,
    queryFn: listResendAccounts,
    enabled: isAdmin,
  })

  if (!authLoading && !isAdmin) {
    return (
      <PageContainer>
        <PageHeader title="Email senders" />
        <p className="text-sm text-muted-foreground">Only Notify admins can manage email senders.</p>
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <PageHeader
        title="Email senders"
        description="From addresses Notify can send email as. Senders pick one with email_sender_id, or get the default."
        action={
          <Button onClick={() => setEditing("new")}>
            <Plus className="size-4" />
            Add sender
          </Button>
        }
      />

      {error && (
        <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {apiErrorMessage(error, "Failed to load email senders.")}
        </div>
      )}

      {accounts?.length === 0 && (
        <div className="mb-4 rounded-lg border border-border/60 bg-muted/40 px-3 py-2 text-sm">
          Senders need a Resend account to send through.{" "}
          <Link to="/resend-accounts" className="font-medium text-primary underline-offset-4 hover:underline">
            Add a Resend account
          </Link>{" "}
          first.
        </div>
      )}

      <div className="space-y-3">
        {(isLoading || authLoading) && <Skeleton className="h-24 w-full" />}
        {senders?.length === 0 && (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
              No email senders yet. Email deliveries will fail until one is added.
            </CardContent>
          </Card>
        )}
        {senders?.map((sender) => (
          <EmailSenderRow
            key={sender.id}
            sender={sender}
            account={accounts?.find((account) => account.id === sender.resend_account_id)}
            onEdit={() => setEditing(sender)}
          />
        ))}
      </div>

      <EmailSenderDialog
        key={editing === "new" ? "new" : editing?.id ?? "closed"}
        sender={editing === "new" ? null : editing}
        accounts={accounts ?? []}
        open={editing !== null}
        onClose={() => setEditing(null)}
      />
    </PageContainer>
  )
}

function EmailSenderRow({
  sender,
  account,
  onEdit,
}: {
  sender: EmailSender
  account?: ResendAccount
  onEdit: () => void
}) {
  const queryClient = useQueryClient()
  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: EMAIL_SENDERS_KEY }),
      queryClient.invalidateQueries({ queryKey: RESEND_ACCOUNTS_KEY }),
    ])

  const update = useMutation({
    mutationFn: (input: EmailSenderInput) => updateEmailSender(sender.id, input),
    onSuccess: invalidate,
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to update sender.")),
  })
  const remove = useMutation({
    mutationFn: () => deleteEmailSender(sender.id),
    onSuccess: () => {
      toast.success(`Deleted ${sender.name}`)
      return invalidate()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to delete sender.")),
  })

  async function copyID() {
    try {
      await navigator.clipboard.writeText(sender.id)
      toast.success("Sender ID copied")
    } catch {
      toast.error("Could not copy to clipboard")
    }
  }

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <Mail className="size-5" />
          </div>
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium">{sender.name}</span>
              {sender.is_default && <Badge>Default</Badge>}
              {!sender.enabled && <Badge variant="destructive">Disabled</Badge>}
            </div>
            <div className="break-all text-sm">{formatEmailFrom(sender)}</div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1">
                <KeyRound className="size-3" />
                {account?.name ?? "unknown account"}
                {account && !account.enabled && " (disabled)"}
              </span>
              {sender.reply_to && (
                <span className="inline-flex items-center gap-1 break-all">
                  <CornerUpLeft className="size-3" />
                  {sender.reply_to}
                </span>
              )}
              <button type="button" onClick={copyID} className="inline-flex items-center gap-1 font-mono hover:text-foreground">
                {sender.id}
                <Copy className="size-3" />
              </button>
            </div>
            {sender.description && <p className="text-sm text-muted-foreground">{sender.description}</p>}
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {!sender.is_default && sender.enabled && (
            <Button variant="outline" size="sm" disabled={update.isPending} onClick={() => update.mutate({ is_default: true })}>
              <Star className="size-3.5" />
              Make default
            </Button>
          )}
          {!sender.is_default && (
            <Button variant="outline" size="sm" disabled={update.isPending} onClick={() => update.mutate({ enabled: !sender.enabled })}>
              {sender.enabled ? "Disable" : "Enable"}
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={onEdit}>
            <Pencil className="size-3.5" />
            Edit
          </Button>
          <ConfirmDialog
            trigger={
              <Button variant="destructive" size="sm">
                <Trash2 className="size-3.5" />
                Delete
              </Button>
            }
            title={`Delete ${sender.name}?`}
            description="Queued emails assigned to this sender will fail. Senders passing its ID will be rejected."
            confirmLabel="Delete"
            isPending={remove.isPending}
            onConfirm={async () => {
              await remove.mutateAsync()
            }}
          />
        </div>
      </CardContent>
    </Card>
  )
}

function EmailSenderDialog({
  sender,
  accounts,
  open,
  onClose,
}: {
  sender: EmailSender | null
  accounts: ResendAccount[]
  open: boolean
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(sender?.name ?? "")
  const [accountID, setAccountID] = useState(sender?.resend_account_id ?? (accounts.length === 1 ? accounts[0].id : ""))
  const [description, setDescription] = useState(sender?.description ?? "")
  const [fromName, setFromName] = useState(sender?.from_name ?? "")
  const [fromAddress, setFromAddress] = useState(sender?.from_address ?? "")
  const [replyTo, setReplyTo] = useState(sender?.reply_to ?? "")

  const save = useMutation({
    mutationFn: () => {
      const input: EmailSenderInput = {
        name,
        description,
        resend_account_id: accountID,
        from_name: fromName,
        from_address: fromAddress.trim(),
        reply_to: replyTo.trim(),
      }
      return sender ? updateEmailSender(sender.id, input) : createEmailSender(input)
    },
    onSuccess: async (saved) => {
      toast.success(sender ? `Updated ${saved.name}` : `Added ${saved.name}`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: EMAIL_SENDERS_KEY }),
        queryClient.invalidateQueries({ queryKey: RESEND_ACCOUNTS_KEY }),
      ])
      onClose()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to save sender.")),
  })

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!accountID) {
      toast.error("Choose a Resend account.")
      return
    }
    save.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{sender ? `Edit ${sender.name}` : "Add email sender"}</DialogTitle>
            <DialogDescription>
              The from address must be on a domain verified in the chosen Resend account, matched exactly (a subdomain
              counts as its own domain). Set a reply-to if the from domain doesn't receive mail.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="sender-name">Name</Label>
            <Input id="sender-name" value={name} onChange={(event) => setName(event.target.value)} maxLength={64} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="sender-account">Resend account</Label>
            <Select value={accountID} onValueChange={setAccountID}>
              <SelectTrigger id="sender-account" className="w-full">
                <SelectValue placeholder="Choose an account" />
              </SelectTrigger>
              <SelectContent>
                {accounts.map((account) => (
                  <SelectItem key={account.id} value={account.id}>
                    {account.name}
                    {!account.enabled && " (disabled)"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="sender-from-name">From name</Label>
              <Input
                id="sender-from-name"
                value={fromName}
                onChange={(event) => setFromName(event.target.value)}
                maxLength={100}
                placeholder="Gaucho Racing"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sender-from-address">From address</Label>
              <Input
                id="sender-from-address"
                type="email"
                value={fromAddress}
                onChange={(event) => setFromAddress(event.target.value)}
                placeholder="team@notify.gauchoracing.com"
                required
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="sender-reply-to">Reply-to</Label>
            <Input
              id="sender-reply-to"
              type="email"
              value={replyTo}
              onChange={(event) => setReplyTo(event.target.value)}
              placeholder="team@gauchoracing.com"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="sender-description">Description</Label>
            <Textarea
              id="sender-description"
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
              {save.isPending ? "Saving" : sender ? "Save" : "Add sender"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
