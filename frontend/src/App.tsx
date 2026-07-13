// Top-level router. Feature pages are lazy-loaded so the initial
// bundle stays small and each stage only adds the chunks it needs.
import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import ComingSoon from './components/ComingSoon'

const HomePage = lazy(() => import('./pages/home/HomePage'))

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Suspense fallback={<div className="py-24 text-center text-muted">…</div>}>
          <Routes>
            <Route path="/" element={<HomePage />} />
            {/* Each feature lands its own page as its stage ships.
                Until then they share the ComingSoon placeholder. */}
            <Route path="/bazi" element={<ComingSoon titleKey="nav.bazi" />} />
            <Route path="/dream" element={<ComingSoon titleKey="nav.dream" />} />
            <Route path="/zodiac" element={<ComingSoon titleKey="nav.zodiac" />} />
            <Route path="/huangli" element={<ComingSoon titleKey="nav.huangli" />} />
            <Route path="/constellation" element={<ComingSoon titleKey="nav.constellation" />} />
            <Route path="/tarot" element={<ComingSoon titleKey="nav.tarot" />} />
            <Route path="/compatibility" element={<ComingSoon titleKey="nav.compatibility" />} />
            <Route path="/account" element={<ComingSoon titleKey="nav.account" />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </Layout>
    </BrowserRouter>
  )
}
