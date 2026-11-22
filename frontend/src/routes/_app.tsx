import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

import { meQuery } from '../api/queries'
import { AppSidebar } from '../components/AppSidebar'

/** Pathless layout guarding every authenticated page and adding the sidebar. */
export const Route = createFileRoute('/_app')({
  beforeLoad: async ({ context, location }) => {
    const user = await context.queryClient.ensureQueryData(meQuery)
    if (user === null) throw redirect({ to: '/login', search: { redirect: location.href } })
    return { user }
  },
  component: AppLayout,
})

function AppLayout() {
  return (
    <>
      <AppSidebar />
      <main className="md:pl-60">
        <div className="mx-auto max-w-5xl px-4 py-10 md:px-8">
          <Outlet />
        </div>
      </main>
    </>
  )
}
