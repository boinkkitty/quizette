import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { meQuery, providersQuery } from '../api/queries'
import { AuthForm } from '../components/AuthForm'

export const authSearchSchema = z.object({
  redirect: z.string().startsWith('/').catch('/').default('/'),
  error: z.string().optional(),
})

export const Route = createFileRoute('/login')({
  validateSearch: authSearchSchema,
  beforeLoad: async ({ context, search }) => {
    // Fetch the sign-in providers alongside the session check, so the buttons render with the
    // form instead of pushing it down when they arrive. prefetchQuery never throws: without
    // providers the form still works.
    const [user] = await Promise.all([
      context.queryClient.ensureQueryData(meQuery),
      context.queryClient.prefetchQuery(providersQuery),
    ])
    if (user !== null) throw redirect({ to: search.redirect })
  },
  component: function LoginPage() {
    const { redirect: to, error } = Route.useSearch()
    return <AuthForm mode="login" redirect={to} oauthError={error} />
  },
})
