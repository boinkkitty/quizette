import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import {
  BookOpenCheckIcon,
  HistoryIcon,
  LayoutDashboardIcon,
  ListChecksIcon,
  LogOutIcon,
  type LucideIcon,
  MenuIcon,
  UploadIcon,
  XIcon,
} from 'lucide-react'
import { useEffect, useState } from 'react'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { meQuery, useLogout } from '../api/queries'

interface NavItem {
  to: '/' | '/upload' | '/quizzes' | '/history'
  label: string
  icon: LucideIcon
  /** Match only the exact path (the dashboard is `/`, which prefixes everything). */
  exact?: boolean
  /** The main call to action: drawn as a solid blue button instead of a plain link. */
  primary?: boolean
}

const navItems: readonly NavItem[] = [
  { to: '/upload', label: 'Upload', icon: UploadIcon, primary: true },
  { to: '/', label: 'Dashboard', icon: LayoutDashboardIcon, exact: true },
  { to: '/quizzes', label: 'Quizzes', icon: ListChecksIcon },
  { to: '/history', label: 'History', icon: HistoryIcon },
]

const linkBase = 'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors'

/** Signed-in navigation: brand on top, page links, then the user and Sign out at the bottom. */
function SidebarContent({ onNavigate }: { onNavigate?: () => void }) {
  const { data: user } = useQuery(meQuery)
  const logout = useLogout()
  const navigate = useNavigate()
  return (
    <div className="flex h-full flex-col gap-6 p-4">
      <Link to="/" onClick={onNavigate} className="flex items-center gap-2 px-3 pt-2 text-lg font-semibold tracking-tight text-primary">
        <BookOpenCheckIcon className="size-5" aria-hidden />
        Quizette
      </Link>

      <nav aria-label="Main" className="flex flex-1 flex-col gap-1">
        {navItems.map(({ to, label, icon: Icon, exact, primary }) => (
          <Link
            key={label}
            to={to}
            onClick={onNavigate}
            activeOptions={{ exact: exact === true }}
            className={cn(
              linkBase,
              primary === true
                ? 'mb-3 justify-center bg-primary py-2.5 font-semibold text-primary-foreground shadow-sm hover:bg-primary/90'
                : 'text-muted-foreground hover:bg-secondary hover:text-foreground',
            )}
            activeProps={{
              className: primary === true ? 'ring-3 ring-primary/30' : 'bg-secondary text-primary hover:text-primary',
              'aria-current': 'page',
            }}
          >
            <Icon className="size-4" aria-hidden />
            {label}
          </Link>
        ))}
      </nav>

      <div className="border-t pt-4">
        {user !== null && user !== undefined && (
          <p className="truncate px-3 pb-2 text-xs text-muted-foreground" title={user.email}>
            {user.name !== '' ? user.name : user.email}
          </p>
        )}
        <Button
          variant="ghost"
          className="w-full justify-start gap-3 px-3 text-muted-foreground"
          disabled={logout.isPending}
          onClick={() => {
            logout.mutate(undefined, { onSettled: () => void navigate({ to: '/login' }) })
          }}
        >
          <LogOutIcon aria-hidden /> Sign out
        </Button>
      </div>
    </div>
  )
}

/**
 * App navigation. On md+ screens it is a fixed left sidebar; on smaller screens a
 * top bar opens the same content as a drawer.
 */
export function AppSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  // Remember the page the drawer was opened on: navigating anywhere (including
  // back/forward) closes it without an effect.
  const [openedOn, setOpenedOn] = useState<string | null>(null)
  const open = openedOn === pathname
  const setOpen = (next: boolean): void => {
    setOpenedOn(next ? pathname : null)
  }

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') setOpenedOn(null)
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <>
      <aside className="fixed inset-y-0 left-0 z-20 hidden w-60 border-r bg-background md:block">
        <SidebarContent />
      </aside>

      <header className="sticky top-0 z-20 flex h-14 items-center justify-between border-b bg-background/95 px-4 backdrop-blur md:hidden">
        <Link to="/" className="flex items-center gap-2 font-semibold tracking-tight text-primary">
          <BookOpenCheckIcon className="size-5" aria-hidden />
          Quizette
        </Link>
        <Button variant="ghost" size="icon" aria-label="Open menu" aria-expanded={open} onClick={() => setOpen(true)}>
          <MenuIcon aria-hidden />
        </Button>
      </header>

      {open && (
        <div className="fixed inset-0 z-30 md:hidden" role="dialog" aria-modal="true" aria-label="Menu">
          <button type="button" aria-label="Close menu" className="absolute inset-0 bg-black/30" onClick={() => setOpen(false)} />
          <aside className="relative h-full w-64 bg-background shadow-lg">
            <Button variant="ghost" size="icon" aria-label="Close menu" className="absolute top-3 right-3" onClick={() => setOpen(false)}>
              <XIcon aria-hidden />
            </Button>
            <SidebarContent onNavigate={() => setOpen(false)} />
          </aside>
        </div>
      )}
    </>
  )
}
