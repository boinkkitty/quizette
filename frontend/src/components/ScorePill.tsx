import { cn } from '@/lib/utils'

import { formatPct } from '../lib/format'

/** A score percentage coloured by band: 80% and up green, 50% and up amber, below red. */
export function ScorePill({ pct, className }: { pct: number; className?: string }) {
  return (
    <span
      className={cn(
        'shrink-0 rounded-full px-2.5 py-1 text-sm font-semibold',
        pct >= 80
          ? 'bg-green-50 text-green-700'
          : pct >= 50
            ? 'bg-amber-50 text-amber-800'
            : 'bg-red-50 text-red-700',
        className,
      )}
    >
      {formatPct(pct)}
    </span>
  )
}
