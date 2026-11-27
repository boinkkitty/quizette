import { EllipsisVerticalIcon, Trash2Icon } from 'lucide-react'
import { DropdownMenu } from 'radix-ui'
import { useState, type ReactNode } from 'react'

import { Button } from '@/components/ui/button'

import { ConfirmDialog } from './ConfirmDialog'

/**
 * DeleteAction is a page's Delete, kept quieter than its primary actions: a ghost button on
 * desktop, and an item in a "More actions" menu on phones. Both open one confirmation dialog.
 * Put it after the page's other actions.
 */
export function DeleteAction({
  label,
  title,
  description,
  disabled = false,
  onConfirm,
}: {
  /** The button, menu item and confirm label, e.g. "Delete topic". */
  label: string
  title: string
  description: ReactNode
  disabled?: boolean
  onConfirm: () => void
}) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button
        variant="ghost"
        size="lg"
        disabled={disabled}
        onClick={() => setOpen(true)}
        className="hidden text-muted-foreground hover:text-destructive md:inline-flex"
      >
        {label}
      </Button>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <Button
            variant="outline"
            size="icon"
            aria-label="More actions"
            disabled={disabled}
            className="size-11 md:hidden"
          >
            <EllipsisVerticalIcon aria-hidden />
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={4}
            className="z-50 min-w-44 rounded-lg border bg-popover p-1 text-popover-foreground shadow-md"
          >
            <DropdownMenu.Item
              onSelect={() => setOpen(true)}
              className="flex min-h-11 cursor-default items-center gap-2 rounded-md px-3 text-sm text-destructive outline-none select-none data-highlighted:bg-destructive/10"
            >
              <Trash2Icon className="size-4" aria-hidden />
              {label}
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
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
