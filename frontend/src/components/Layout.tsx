// Page chrome shared by every route: navbar, content slot, footer.
import type { ReactNode } from 'react'
import Navbar from './Navbar'
import Footer from './Footer'
import SafetyNotice from './SafetyNotice'

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-full flex-col">
      <Navbar />
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-8">{children}</main>
      <SafetyNotice />
      <Footer />
    </div>
  )
}
