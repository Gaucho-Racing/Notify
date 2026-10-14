import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, MessageSquare, Pencil, Phone, Plus, Star, Trash2 } from "lucide-react"
import { useState, type FormEvent } from "react"
import { Link } from "react-router-dom"
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { apiErrorMessage } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import {
  createSMSSender,
  deleteSMSSender,
  listSMSSenders,
  listTwilioAccounts,
  listTwilioSenderOptions,
  TWILIO_ACCOUNTS_KEY,
  updateSMSSender,
  type SMSSender,
  type SMSSenderInput,
  type TwilioAccount,
} from "@/lib/notify"
import { cn } from "@/lib/utils"

const SMS_SENDERS_KEY = ["smsSenders"]

export default function SMSSendersPage() {
  const { isAdmin, isLoading: authLoading } = useAuth()
  const [editing, setEditing] = useState<SMSSender | "new" | null>(null)
  const { data: senders, isLoading, error } = useQuery({
    queryKey: SMS_SENDERS_KEY,
    queryFn: listSMSSenders,
    enabled: isAdmin,
  })
  const { data: accounts } = useQuery({
    queryKey: TWILIO_ACCOUNTS_KEY,
    queryFn: listTwilioAccounts,
    enabled: isAdmin,
  })

  if (!authLoading && !isAdmin) {
    return (
      <PageContainer>
        <PageHeader title="SMS senders" />
        <p className="text-sm text-muted-foreground">Only Notify admins can manage SMS senders.</p>
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <PageHeader
        eyebrow={<ChannelTag channel="SMS" />}
        title="SMS senders"
        description="Numbers Notify can text from. Senders pick one with sms_sender_id, or get the default."
        action={
          <Button onClick={() => setEditing("new")}>
            <Plus className="size-4" />
            Add sender
          </Button>
        }
      />

      {error && (
        <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {apiErrorMessage(error, "Failed to load SMS senders.")}
        </div>
      )}

      {accounts?.length === 0 && (
        <div className="mb-4 rounded-lg border border-border/60 bg-muted/40 px-3 py-2 text-sm">
          Senders need a Twilio account to send through.{" "}
          <Link to="/twilio-accounts" className="font-medium text-primary underline-offset-4 hover:underline">
            Add a Twilio account
          </Link>{" "}
          first.
        </div>
      )}

      <div className="space-y-3">
        {(isLoading || authLoading) && <Skeleton className="h-24 w-full" />}
        {senders?.length === 0 && (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
              No SMS senders yet. SMS deliveries will fail until one is added.
            </CardContent>
          </Card>
        )}
        {senders?.map((sender) => (
          <SMSSenderRow
            key={sender.id}
            sender={sender}
            account={accounts?.find((account) => account.id === sender.twilio_account_id)}
            onEdit={() => setEditing(sender)}
          />
        ))}
      </div>

      <SMSSenderDialog
        key={editing === "new" ? "new" : editing?.id ?? "closed"}
        sender={editing === "new" ? null : editing}
        accounts={accounts ?? []}
        open={editing !== null}
        onClose={() => setEditing(null)}
      />
    </PageContainer>
  )
}

function SMSSenderRow({
  sender,
  account,
  onEdit,
}: {
  sender: SMSSender
  account?: TwilioAccount
  onEdit: () => void
}) {
  const queryClient = useQueryClient()
  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: SMS_SENDERS_KEY }),
      queryClient.invalidateQueries({ queryKey: TWILIO_ACCOUNTS_KEY }),
    ])

  const update = useMutation({
    mutationFn: (input: SMSSenderInput) => updateSMSSender(sender.id, input),
    onSuccess: invalidate,
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to update sender.")),
  })
  const remove = useMutation({
    mutationFn: () => deleteSMSSender(sender.id),
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
    <Card className={cn(!sender.enabled && "opacity-60")}>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <ChannelTile channel="SMS">
              <MessageSquare className="size-4" />
            </ChannelTile>
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium">{sender.name}</span>
              {sender.is_default && <Badge>Default</Badge>}
              {!sender.enabled && <Badge variant="destructive">Disabled</Badge>}
            </div>
            <div className="break-all font-mono text-sm">
              {sender.from_number || `Messaging service ${sender.messaging_service_sid}`}
            </div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1">
                <Phone className="size-3" />
                {account?.name ?? "unknown account"}
                {account && !account.enabled && " (disabled)"}
              </span>
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
            description="Queued texts assigned to this sender will fail. Senders passing its ID will be rejected."
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

function sourceValue(sender: SMSSender | null) {
  if (sender?.from_number) return `number:${sender.from_number}`
  if (sender?.messaging_service_sid) return `service:${sender.messaging_service_sid}`
  return ""
}

function SMSSenderDialog({
  sender,
  accounts,
  open,
  onClose,
}: {
  sender: SMSSender | null
  accounts: TwilioAccount[]
  open: boolean
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(sender?.name ?? "")
  const [accountID, setAccountID] = useState(sender?.twilio_account_id ?? (accounts.length === 1 ? accounts[0].id : ""))
  const [source, setSource] = useState(sourceValue(sender))
  const [description, setDescription] = useState(sender?.description ?? "")
  const { data: options, isLoading: optionsLoading, error: optionsError } = useQuery({
    queryKey: [...TWILIO_ACCOUNTS_KEY, accountID, "senders"],
    queryFn: () => listTwilioSenderOptions(accountID),
    enabled: open && accountID !== "",
  })
  const currentSourceListed =
    source === "" ||
    options?.phone_numbers.some((number) => `number:${number.phone_number}` === source) ||
    options?.messaging_services.some((service) => `service:${service.sid}` === source)

  const save = useMutation({
    mutationFn: () => {
      const separator = source.indexOf(":")
      const kind = source.slice(0, separator)
      const value = source.slice(separator + 1)
      const input: SMSSenderInput = {
        name,
        description,
        twilio_account_id: accountID,
        from_number: kind === "number" ? value : "",
        messaging_service_sid: kind === "service" ? value : "",
      }
      return sender ? updateSMSSender(sender.id, input) : createSMSSender(input)
    },
    onSuccess: async (saved) => {
      toast.success(sender ? `Updated ${saved.name}` : `Added ${saved.name}`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: SMS_SENDERS_KEY }),
        queryClient.invalidateQueries({ queryKey: TWILIO_ACCOUNTS_KEY }),
      ])
      onClose()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to save sender.")),
  })

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!accountID) {
      toast.error("Choose a Twilio account.")
      return
    }
    if (!source) {
      toast.error("Choose a number or messaging service to send from.")
      return
    }
    save.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{sender ? `Edit ${sender.name}` : "Add SMS sender"}</DialogTitle>
            <DialogDescription>
              Send from one Twilio number, or from a messaging service and let Twilio pick from its sender pool. US numbers
              need A2P 10DLC or toll-free verification in Twilio before carriers will deliver.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="sms-name">Name</Label>
            <Input id="sms-name" value={name} onChange={(event) => setName(event.target.value)} maxLength={64} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="sms-account">Twilio account</Label>
            <Select
              value={accountID}
              onValueChange={(value) => {
                setAccountID(value)
                setSource("")
              }}
            >
              <SelectTrigger id="sms-account" className="w-full">
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
          <div className="space-y-2">
            <Label htmlFor="sms-source">Send from</Label>
            <Select value={source} onValueChange={setSource} disabled={!accountID || optionsLoading}>
              <SelectTrigger id="sms-source" className="w-full">
                <SelectValue placeholder={optionsLoading ? "Loading numbers…" : "Choose a number or messaging service"} />
              </SelectTrigger>
              <SelectContent>
                {!currentSourceListed && <SelectItem value={source}>{source.slice(source.indexOf(":") + 1)}</SelectItem>}
                {options?.phone_numbers
                  .filter((number) => number.sms)
                  .map((number) => (
                    <SelectItem key={number.phone_number} value={`number:${number.phone_number}`}>
                      {number.phone_number}
                      {number.friendly_name && number.friendly_name !== number.phone_number && ` · ${number.friendly_name}`}
                    </SelectItem>
                  ))}
                {options?.messaging_services.map((service) => (
                  <SelectItem key={service.sid} value={`service:${service.sid}`}>
                    {service.friendly_name} · messaging service
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {optionsError && (
              <p className="text-sm text-destructive">{apiErrorMessage(optionsError, "Failed to load numbers.")}</p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="sms-description">Description</Label>
            <Textarea
              id="sms-description"
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
