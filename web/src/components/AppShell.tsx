import { Outlet } from "react-router-dom"

import { AppFooter } from "@/components/AppFooter"
import { AppHeader } from "@/components/AppHeader"
import { AppSidebar } from "@/components/AppSidebar"
import { TooltipProvider } from "@/components/ui/tooltip"

export function AppShell() {
  return (
    <TooltipProvider>
      <div className="grid min-h-svh bg-background text-foreground lg:grid-cols-[264px_minmax(0,1fr)]">
        <AppSidebar />
        <div className="flex min-h-svh min-w-0 flex-col bg-background">
          <AppHeader />
          <main className="flex-1">
            <Outlet />
          </main>
          <AppFooter />
        </div>
      </div>
    </TooltipProvider>
  )
}
