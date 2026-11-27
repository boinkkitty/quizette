import { createFileRoute, redirect } from '@tanstack/react-router'

import { meQuery, providersQuery } from '../api/queries'
import { AuthForm } from '../components/AuthForm'
import { authSearchSchema } from './login'

export const Route = createFileRoute('/signup')({
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
  component: function SignupPage() {
    const { redirect: to } = Route.useSearch()
    return <AuthForm mode="signup" redirect={to} />
  },
})
