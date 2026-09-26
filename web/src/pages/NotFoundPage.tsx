import { Link } from "react-router-dom"

import { Button } from "@/components/ui/button"

export default function NotFoundPage() {
  return (
    <main className="flex min-h-svh items-center justify-center px-4 py-12">
      <div className="space-y-4 text-center">
        <h1 className="text-2xl font-semibold">Not found</h1>
        <Button asChild variant="outline">
          <Link to="/dashboard">Back to dashboard</Link>
        </Button>
      </div>
    </main>
  )
}
