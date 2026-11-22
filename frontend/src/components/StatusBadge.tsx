import { Badge } from '@/components/ui/badge'

import type { ChapterStatus, JobStatus } from '../api/schemas'
import { Spinner } from './Spinner'

export function StatusBadge({ status }: { status: ChapterStatus | JobStatus }) {
  switch (status) {
    case 'pending':
    case 'queued':
    case 'running':
      return (
        <Badge variant="info">
          <Spinner className="size-3" /> Generating
        </Badge>
      )
    case 'ready':
    case 'succeeded':
      return <Badge variant="success">Ready</Badge>
    case 'failed':
      return <Badge variant="destructive">Failed</Badge>
  }
}
