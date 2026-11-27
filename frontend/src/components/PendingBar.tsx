import { useRouterState } from '@tanstack/react-router'

/**
 * PendingBar is a thin bar across the top of the window while the router loads the next page, so
 * a click answers at once even when the API is waking up. It fades in after a short delay, so fast
 * navigations don't flash it.
 */
export function PendingBar() {
  const pending = useRouterState({ select: (s) => s.status === 'pending' })
  if (!pending) return null
  return (
    <div
      role="progressbar"
      aria-label="Loading page"
      className="pointer-events-none fixed inset-x-0 top-0 z-50 h-0.5 overflow-hidden bg-primary/20 animate-in fade-in fill-mode-both delay-150"
    >
      <div className="h-full w-1/3 animate-[pending-bar_1.2s_ease-in-out_infinite] bg-primary" />
    </div>
  )
}
