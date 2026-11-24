import { CircleAlertIcon } from 'lucide-react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

import { errorText } from '../lib/errors'

export function ErrorMessage({
  error,
  title = 'Something went wrong',
}: {
  error: unknown
  title?: string
}) {
  return (
    <Alert variant="destructive">
      <CircleAlertIcon />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{errorText(error)}</AlertDescription>
    </Alert>
  )
}
