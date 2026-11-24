import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Link, Outlet } from '@tanstack/react-router'

import { Toaster } from '@/components/ui/sonner'

export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  notFoundComponent: () => (
    <div className="px-4 py-20 text-center">
      <p className="text-lg font-medium">Page not found</p>
      <Link to="/" className="mt-2 inline-block text-sm text-primary hover:underline">
        Back to dashboard
      </Link>
    </div>
  ),
})

/** Shared by every page. Signed-in pages add the sidebar in the `_app` layout. */
function RootLayout() {
  return (
    <>
      <div className="min-h-screen bg-background">
        <Outlet />
      </div>
      <Toaster position="bottom-right" />
    </>
  )
}
