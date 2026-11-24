import { QueryClientProvider } from '@tanstack/react-query'
import { createMemoryHistory, RouterProvider } from '@tanstack/react-router'
import { render } from '@testing-library/react'

import { ServerWakeBanner } from '../components/ServerWakeBanner'
import { createAppRouter } from '../router'
import { testQueryClient } from './render'

/** Renders the real router (all routes, guards and loaders) at path. */
export function renderApp(path: string) {
  const queryClient = testQueryClient()
  const router = createAppRouter(queryClient, createMemoryHistory({ initialEntries: [path] }))
  const reloadRoutes = () => router.invalidate()
  const result = render(
    <QueryClientProvider client={queryClient}>
      <ServerWakeBanner onAwake={reloadRoutes} />
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { ...result, router, queryClient }
}
