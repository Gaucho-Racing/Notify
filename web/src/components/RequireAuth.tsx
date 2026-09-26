import { Navigate, Outlet, useLocation } from "react-router-dom"

import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/lib/auth"

export function RequireAuth() {
  const location = useLocation()
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-background px-4">
        <div className="w-full max-w-sm space-y-3">
          <Skeleton className="mx-auto size-10 rounded-lg" />
          <Skeleton className="h-3 w-full rounded-full" />
          <Skeleton className="mx-auto h-3 w-2/3 rounded-full" />
        </div>
      </main>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/auth/login" state={{ from: location }} replace />
  }
  return <Outlet />
}
