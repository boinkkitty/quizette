import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { CircleAlertIcon } from 'lucide-react'
import { type FormEvent, useState } from 'react'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'

import { providersQuery, useAuthMutation } from '../api/queries'
import type { OAuthProvider } from '../api/schemas'
import { ErrorMessage } from './ErrorMessage'
import { Field } from './Field'
import { Spinner } from './Spinner'

const providerLabel: Record<OAuthProvider, string> = { google: 'Continue with Google', github: 'Continue with GitHub' }

export function AuthForm({ mode, redirect, oauthError }: { mode: 'login' | 'signup'; redirect: string; oauthError?: string | undefined }) {
  const navigate = useNavigate()
  const auth = useAuthMutation(mode)
  const { data: providers } = useQuery(providersQuery)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    auth.mutate(mode === 'signup' ? { email, password, name } : { email, password }, {
      onSuccess: () => void navigate({ to: redirect }),
    })
  }

  return (
    <div className="mx-auto max-w-sm px-4 pt-16 pb-10">
      <Card>
        <CardHeader className="text-center">
          <CardTitle>
            <h1 className="text-2xl font-semibold tracking-tight">{mode === 'login' ? 'Log in to Quizette' : 'Create your account'}</h1>
          </CardTitle>
          <CardDescription>{mode === 'login' ? 'Welcome back. Pick up where you left off.' : 'Turn any PDF into a practice quiz.'}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
        {oauthError !== undefined && (
          <Alert variant="destructive">
            <CircleAlertIcon />
            <AlertDescription>{oauthError}</AlertDescription>
          </Alert>
        )}
        {(providers?.providers ?? []).map((p) => (
          <Button key={p} variant="outline" size="lg" className="w-full" asChild>
            <a href={`/api/auth/${p}/start`}>{providerLabel[p]}</a>
          </Button>
        ))}
        {(providers?.providers.length ?? 0) > 0 && (
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <Separator className="flex-1" /> or <Separator className="flex-1" />
          </div>
        )}
        <form onSubmit={onSubmit} className="space-y-4">
          {mode === 'signup' && (
            <Field label="Name">
{(c) => (
<Input {...c} value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" />
)}
</Field>
          )}
          <Field label="Email">
{(c) => (
<Input {...c} type="email" required value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
)}
</Field>
          <Field label="Password" {...(mode === 'signup' ? { hint: '8–72 characters' } : {})}>
{(c) => (
<Input {...c}
              type="password"
              required
              minLength={mode === 'signup' ? 8 : undefined}
              maxLength={72}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={mode === 'signup' ? 'new-password' : 'current-password'}
            />
)}
</Field>
          {auth.isError && <ErrorMessage error={auth.error} title={mode === 'login' ? 'Could not log in' : 'Could not sign up'} />}
          <Button type="submit" size="lg" className="w-full" disabled={auth.isPending}>
            {auth.isPending && <Spinner />} {mode === 'login' ? 'Log in' : 'Sign up'}
          </Button>
        </form>
        </CardContent>
      </Card>
      <p className="mt-4 text-center text-sm text-muted-foreground">
        {mode === 'login' ? (
          <>
            No account?{' '}
            <Link to="/signup" search={{ redirect }} className="font-medium text-primary hover:underline">
              Sign up
            </Link>
          </>
        ) : (
          <>
            Have an account?{' '}
            <Link to="/login" search={{ redirect }} className="font-medium text-primary hover:underline">
              Log in
            </Link>
          </>
        )}
      </p>
    </div>
  )
}
