// PROTOTYPE: throwaway variant switcher for UI prototypes. Never merge to main.
import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { useEffect } from 'react'

export interface PrototypeVariant {
  key: string
  name: string
}

/**
 * PrototypeSwitcher is a floating pill that cycles `?variant=` with arrows or ←/→.
 * It sits at the top so it never hides bottom bars, which are part of what is being judged.
 */
export function PrototypeSwitcher({
  variants,
  current,
  onChange,
}: {
  variants: readonly PrototypeVariant[]
  current: string
  onChange: (key: string) => void
}) {
  const i = Math.max(
    0,
    variants.findIndex((v) => v.key === current),
  )
  const go = (d: number) =>
    onChange(variants[(i + d + variants.length) % variants.length]?.key ?? current)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null
      if (t?.closest('input, textarea, select, [contenteditable]')) return
      if (e.key === 'ArrowLeft') go(-1)
      if (e.key === 'ArrowRight') go(1)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  if (!import.meta.env.DEV) return null
  const v = variants[i] ?? { key: current, name: '?' }
  return (
    <div className="fixed top-3 left-1/2 z-50 flex -translate-x-1/2 items-center gap-1 rounded-full bg-zinc-900 px-1 py-1 text-sm text-white shadow-lg ring-2 ring-fuchsia-500">
      <button
        type="button"
        className="rounded-full p-1.5 hover:bg-white/15"
        onClick={() => go(-1)}
        aria-label="Previous variant"
      >
        <ChevronLeftIcon className="size-4" />
      </button>
      <span className="px-2 font-medium whitespace-nowrap">
        {v.key} ({v.name})
      </span>
      <button
        type="button"
        className="rounded-full p-1.5 hover:bg-white/15"
        onClick={() => go(1)}
        aria-label="Next variant"
      >
        <ChevronRightIcon className="size-4" />
      </button>
    </div>
  )
}
