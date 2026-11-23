import { createFileRoute, redirect } from '@tanstack/react-router'

import { meQuery } from '../api/queries'
import { AuthForm } from '../components/AuthForm'
import { authSearchSchema } from './login'

export const Route = createFileRoute('/signup')({
  validateSearch: authSearchSchema,
  beforeLoad: async ({ context, search }) => {
    const user = await context.queryClient.ensureQueryData(meQuery)
    if (user !== null) throw redirect({ to: search.redirect })
  },
  component: function SignupPage() {
    const { redirect: to } = Route.useSearch()
    return <AuthForm mode="signup" redirect={to} />
  },
})
