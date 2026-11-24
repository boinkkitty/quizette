import './styles.css'

import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { ServerWakeBanner } from './components/ServerWakeBanner'
import { createAppRouter, createQueryClient } from './router'

const queryClient = createQueryClient()
const router = createAppRouter(queryClient)
// Route loaders may have exhausted their retries during a cold start.
const reloadRoutes = () => router.invalidate()

const root = document.getElementById('root')
if (root === null) throw new Error('missing #root')

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      {/* Outside the router so the wake-up check starts before route loaders run. */}
      <ServerWakeBanner onAwake={reloadRoutes} />
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
