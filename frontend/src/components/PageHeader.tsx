import type { ReactNode } from 'react'

/** Classes for a page's "← Parent" link: a 44 px tap target on phones, plain text on desktop. */
export const backLinkClass =
  'inline-flex min-h-11 items-center break-words hover:text-primary hover:underline md:min-h-0'

/** PageHeader is a page's h1 with an optional subtitle (often its back link) and actions. */
export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string
  subtitle?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold tracking-tight break-words">{title}</h1>
        {subtitle !== undefined && (
          <div className="mt-1 text-sm text-muted-foreground">{subtitle}</div>
        )}
      </div>
      {actions}
    </div>
  )
}
