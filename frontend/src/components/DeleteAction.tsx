import { Trash2Icon } from 'lucide-react'
import { useState, type ReactNode } from 'react'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { ConfirmDialog } from './ConfirmDialog'

/**
 * DeleteAction is a bin icon that opens a confirmation dialog, kept quieter than a page's or
 * row's other actions. It is a 44 px target on phones.
 */
export function DeleteAction({
  label,
  name,
  title,
  description,
  disabled = false,
  onConfirm,
  className,
}: {
  /** What it does, e.g. "Delete topic": the confirm button, and the icon's name and tooltip. */
  label: string
  /** What it deletes, added to the icon's name where a list has several, e.g. "Biology". */
  name?: string
  title: string
  description: ReactNode
  disabled?: boolean
  onConfirm: () => void
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const accessibleName = name === undefined ? label : `${label} ${name}`
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={accessibleName}
        title={label}
        disabled={disabled}
        onClick={() => setOpen(true)}
        className={cn(
          'size-11 text-muted-foreground hover:bg-destructive/10 hover:text-destructive md:size-9',
          className,
        )}
      >
        <Trash2Icon aria-hidden />
      </Button>
      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={title}
        description={description}
        confirmLabel={label}
        destructive
        onConfirm={onConfirm}
      />
    </>
  )
}
