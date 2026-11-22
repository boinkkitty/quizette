import { isApiError } from '../api/client'

export function errorText(err: unknown): string {
  if (isApiError(err)) return err.message
  if (err instanceof Error) return err.message
  return 'Something went wrong.'
}
