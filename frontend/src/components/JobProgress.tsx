import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CircleAlertIcon, CircleCheckIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'

import { jobQuery } from '../api/queries'
import { ErrorMessage } from './ErrorMessage'
import { Spinner } from './Spinner'

const stepText = {
  queued: 'Waiting for a worker…',
  running: 'Reading your PDF and writing questions…',
} as const

/**
 * JobProgress polls a processing job until it finishes. Once it succeeds it shows ready,
 * or by default a link to the chapter.
 */
export function JobProgress({
  jobId,
  chapterId,
  ready,
}: {
  jobId: string
  chapterId: string
  ready?: ReactNode
}) {
  const { data: job, error } = useQuery(jobQuery(jobId))
  if (error !== null) return <ErrorMessage error={error} />
  if (job === undefined) return <Spinner />

  switch (job.status) {
    case 'queued':
    case 'running':
      return (
        <Card>
          <CardContent className="flex items-center gap-3">
            <Spinner className="size-5 text-primary" />
            <div>
              <p className="font-medium">Generating {job.requestedCount} questions</p>
              <p className="text-sm text-muted-foreground">
                {stepText[job.status]} This usually takes 20–60 seconds.
              </p>
            </div>
          </CardContent>
        </Card>
      )
    case 'failed':
      return (
        <Alert variant="destructive">
          <CircleAlertIcon />
          <AlertTitle>Generation failed</AlertTitle>
          <AlertDescription>{job.error ?? 'Please try again.'}</AlertDescription>
        </Alert>
      )
    case 'succeeded':
      return (
        <Alert variant="success">
          <CircleCheckIcon />
          <AlertTitle>Your question bank is ready</AlertTitle>
          <AlertDescription>
            {ready ?? (
              <Link to="/chapters/$chapterId" params={{ chapterId }} className="font-medium">
                Go to chapter →
              </Link>
            )}
          </AlertDescription>
        </Alert>
      )
  }
}
