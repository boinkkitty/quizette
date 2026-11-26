import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

/** A rounded toggle button for choosing one filter out of several. */
export function Chip({
  active,
  deleted = false,
  onClick,
  children,
}: {
  active: boolean
  /** Marks a filter for deleted content: a dashed border while not chosen. */
  deleted?: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        'flex min-h-11 min-w-11 shrink-0 items-center justify-center gap-1.5 rounded-full border px-3 text-sm whitespace-nowrap',
        active ? 'border-primary bg-primary text-primary-foreground' : 'bg-card hover:bg-muted',
        deleted && !active && 'border-dashed text-muted-foreground',
      )}
    >
      {children}
    </button>
  )
}
