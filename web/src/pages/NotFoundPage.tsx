import { Link } from "react-router-dom"

import { NotifyMark } from "@/components/NotifyMark"
import { Eyebrow } from "@/components/Telemetry"
import { Button } from "@/components/ui/button"

export default function NotFoundPage() {
  return (
    <main className="flex min-h-svh items-center justify-center px-4 py-12">
      <div className="flex flex-col items-center gap-4 text-center">
        <NotifyMark className="size-8 text-primary" />
        <Eyebrow>Error 404 · no signal</Eyebrow>
        <h1 className="font-heading text-3xl font-semibold">Nothing on this channel</h1>
        <Button asChild variant="outline">
          <Link to="/dashboard">Back to dashboard</Link>
        </Button>
      </div>
    </main>
  )
}
