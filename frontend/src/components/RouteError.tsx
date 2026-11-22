import { useQueryClient } from '@tanstack/react-query'
import { type ErrorComponentProps, Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { isApiError } from '../api/client'
import { keys } from '../api/queries'
import { EmptyState } from './EmptyState'
import { ErrorMessage } from './ErrorMessage'

export function RouteError({ error }: ErrorComponentProps) {
  if (isApiError(error) && error.status === 401) return <SessionExpired />
  if (isApiError(error) && error.status === 404) {
    return (
      <EmptyState title="We couldn't find that">
        It may have been deleted, or it belongs to another account.{' '}
        <Link to="/" className="font-medium text-primary hover:underline">
          Back to dashboard
        </Link>
      </EmptyState>
    )
  }
  return <ErrorMessage error={error} />
}

/** The session ended (expired or logged out elsewhere): forget it and go to login, then come back. */
function SessionExpired() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  // Capture the destination once: this component stays mounted while the
  // navigation runs, and re-reading the location would nest /login redirects.
  const href = useRouterState({ select: (s) => s.location.href })
  const [redirect] = useState(href)
  useEffect(() => {
    // Clear the user before navigating, or the login page would see the stale
    // user and bounce straight back here.
    qc.setQueryData(keys.me, null)
    void navigate({ to: '/login', search: { redirect }, replace: true })
  }, [qc, navigate, redirect])
  return null
}
