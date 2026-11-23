import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { meQuery } from '../api/queries'
import { AuthForm } from '../components/AuthForm'

export const authSearchSchema = z.object({
  redirect: z.string().startsWith('/').catch('/').default('/'),
  error: z.string().optional(),
})

export const Route = createFileRoute('/login')({
  validateSearch: authSearchSchema,
  beforeLoad: async ({ context, search }) => {
    const user = await context.queryClient.ensureQueryData(meQuery)
    if (user !== null) throw redirect({ to: search.redirect })
  },
  component: function LoginPage() {
    const { redirect: to, error } = Route.useSearch()
    return <AuthForm mode="login" redirect={to} oauthError={error} />
  },
})
