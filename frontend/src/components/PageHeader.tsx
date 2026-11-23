import type { ReactNode } from 'react'

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {subtitle !== undefined && <div className="mt-1 text-sm text-muted-foreground">{subtitle}</div>}
      </div>
      {actions}
    </div>
  )
}
