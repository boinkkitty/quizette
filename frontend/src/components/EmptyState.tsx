import type { ReactNode } from 'react'

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-primary/30 bg-secondary/40 p-10 text-center">
      <p className="font-medium">{title}</p>
      {children !== undefined && <p className="mt-2 text-sm text-muted-foreground">{children}</p>}
    </div>
  )
}
