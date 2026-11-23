import { Loader2Icon } from 'lucide-react'

import { cn } from '@/lib/utils'

/** Indeterminate loading indicator, announced to screen readers as "Loading". */
export function Spinner({ className }: { className?: string }) {
  return (
    <Loader2Icon
      role="status"
      aria-label="Loading"
      className={cn('size-4 animate-spin', className)}
    />
  )
}
