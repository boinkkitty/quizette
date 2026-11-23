import { type ReactNode, useId } from 'react'

import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

export interface FieldControlProps {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
}

/** Label + control + hint/error, wired with htmlFor and aria-describedby. */
export function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string
  hint?: string
  error?: string | undefined
  children: (control: FieldControlProps) => ReactNode
}) {
  const id = useId()
  const descId = `${id}-desc`
  const desc = error ?? hint
  const control: FieldControlProps = { id, ...(desc !== undefined ? { 'aria-describedby': descId } : {}), ...(error !== undefined ? { 'aria-invalid': true } : {}) }
  return (
    // content-start: in a grid row with a taller neighbour (e.g. one with a hint), keep the
    // label and control at the top instead of spreading them over the extra height.
    <div className="grid content-start gap-2">
      <Label htmlFor={id}>{label}</Label>
      {children(control)}
      {desc !== undefined && (
        <p id={descId} className={cn('text-xs', error !== undefined ? 'text-destructive' : 'text-muted-foreground')}>
          {desc}
        </p>
      )}
    </div>
  )
}
