import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Bot, Copy, Pencil, Plus, Star, Trash2 } from "lucide-react"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PageContainer, PageHeader } from "@/components/PageContainer"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
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
  createDiscordBot,
  deleteDiscordBot,
  listDiscordBots,
  updateDiscordBot,
  type DiscordBot,
  type DiscordBotInput,
} from "@/lib/notify"

const DISCORD_BOTS_KEY = ["discordBots"]

export default function DiscordBotsPage() {
  const { isAdmin, isLoading: authLoading } = useAuth()
  const [editing, setEditing] = useState<DiscordBot | "new" | null>(null)
  const { data: bots, isLoading, error } = useQuery({
    queryKey: DISCORD_BOTS_KEY,
    queryFn: listDiscordBots,
    enabled: isAdmin,
  })

  if (!authLoading && !isAdmin) {
    return (
      <PageContainer>
        <PageHeader title="Discord bots" />
        <p className="text-sm text-muted-foreground">Only Notify admins can manage Discord bots.</p>
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <PageHeader
        title="Discord bots"
        description="Bots Notify can send Discord DMs through. Senders pick one with discord_bot_id, or get the default."
        action={
          <Button onClick={() => setEditing("new")}>
            <Plus className="size-4" />
            Add bot
          </Button>
        }
      />

      {error && (
        <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {apiErrorMessage(error, "Failed to load Discord bots.")}
        </div>
      )}

      <div className="space-y-3">
        {(isLoading || authLoading) && <Skeleton className="h-24 w-full" />}
        {bots?.length === 0 && (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
              No Discord bots yet. Discord deliveries will fail until one is added.
            </CardContent>
          </Card>
        )}
        {bots?.map((bot) => <DiscordBotRow key={bot.id} bot={bot} onEdit={() => setEditing(bot)} />)}
      </div>

      <DiscordBotDialog
        key={editing === "new" ? "new" : editing?.id ?? "closed"}
        bot={editing === "new" ? null : editing}
        open={editing !== null}
        onClose={() => setEditing(null)}
      />
    </PageContainer>
  )
}

function DiscordBotRow({ bot, onEdit }: { bot: DiscordBot; onEdit: () => void }) {
  const queryClient = useQueryClient()
  const invalidate = () => queryClient.invalidateQueries({ queryKey: DISCORD_BOTS_KEY })

  const update = useMutation({
    mutationFn: (input: DiscordBotInput) => updateDiscordBot(bot.id, input),
    onSuccess: invalidate,
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to update bot.")),
  })
  const remove = useMutation({
    mutationFn: () => deleteDiscordBot(bot.id),
    onSuccess: () => {
      toast.success(`Deleted ${bot.name}`)
      return invalidate()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to delete bot.")),
  })

  async function copyID() {
    try {
      await navigator.clipboard.writeText(bot.id)
      toast.success("Bot ID copied")
    } catch {
      toast.error("Could not copy to clipboard")
    }
  }

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <Avatar size="lg">
            <AvatarImage src={bot.avatar_url} alt="" />
            <AvatarFallback>
              <Bot className="size-5" />
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium">{bot.name}</span>
              {bot.is_default && <Badge>Default</Badge>}
              {!bot.enabled && <Badge variant="destructive">Disabled</Badge>}
            </div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span>@{bot.discord_username}</span>
              <button type="button" onClick={copyID} className="inline-flex items-center gap-1 font-mono hover:text-foreground">
                {bot.id}
                <Copy className="size-3" />
              </button>
              <span className="font-mono">token ••••{bot.token_hint}</span>
            </div>
            {bot.description && <p className="text-sm text-muted-foreground">{bot.description}</p>}
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {!bot.is_default && bot.enabled && (
            <Button variant="outline" size="sm" disabled={update.isPending} onClick={() => update.mutate({ is_default: true })}>
              <Star className="size-3.5" />
              Make default
            </Button>
          )}
          {!bot.is_default && (
            <Button variant="outline" size="sm" disabled={update.isPending} onClick={() => update.mutate({ enabled: !bot.enabled })}>
              {bot.enabled ? "Disable" : "Enable"}
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
            title={`Delete ${bot.name}?`}
            description="Queued Discord deliveries assigned to this bot will fail. Senders passing its ID will be rejected."
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

function DiscordBotDialog({ bot, open, onClose }: { bot: DiscordBot | null; open: boolean; onClose: () => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(bot?.name ?? "")
  const [description, setDescription] = useState(bot?.description ?? "")
  const [token, setToken] = useState("")

  const save = useMutation({
    mutationFn: () => {
      const input: DiscordBotInput = { name, description }
      if (token.trim()) input.token = token.trim()
      return bot ? updateDiscordBot(bot.id, input) : createDiscordBot(input)
    },
    onSuccess: async (saved) => {
      toast.success(bot ? `Updated ${saved.name}` : `Added ${saved.name} (@${saved.discord_username})`)
      await queryClient.invalidateQueries({ queryKey: DISCORD_BOTS_KEY })
      onClose()
    },
    onError: (error) => toast.error(apiErrorMessage(error, "Failed to save bot.")),
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
            <DialogTitle>{bot ? `Edit ${bot.name}` : "Add Discord bot"}</DialogTitle>
            <DialogDescription>
              The token is checked with Discord, then stored encrypted. It is never shown again. The bot must share a
              server with recipients to DM them.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="bot-name">Name</Label>
            <Input id="bot-name" value={name} onChange={(event) => setName(event.target.value)} maxLength={64} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="bot-description">Description</Label>
            <Textarea
              id="bot-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              maxLength={500}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="bot-token">Bot token</Label>
            <Input
              id="bot-token"
              type="password"
              autoComplete="off"
              value={token}
              onChange={(event) => setToken(event.target.value)}
              placeholder={bot ? `Leave blank to keep ••••${bot.token_hint}` : ""}
              required={!bot}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? "Saving" : bot ? "Save" : "Add bot"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
