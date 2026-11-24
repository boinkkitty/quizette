import { QueryClient } from '@tanstack/react-query'
import { createRouter, type RouterHistory } from '@tanstack/react-router'

import { isApiError } from './api/client'
import { RouteError } from './components/RouteError'
import { routeTree } from './routeTree.gen'

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (count, err) =>
          !(isApiError(err) && err.status >= 400 && err.status < 500) && count < 2,
      },
    },
  })
}

export function createAppRouter(queryClient: QueryClient, history?: RouterHistory) {
  return createRouter({
    routeTree,
    ...(history !== undefined ? { history } : {}),
    context: { queryClient },
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0, // let TanStack Query own caching
    scrollRestoration: true,
    defaultErrorComponent: RouteError,
  })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
