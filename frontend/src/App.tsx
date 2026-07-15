// Top-level router. Feature pages are lazy-loaded so the initial
// bundle stays small and each stage only adds the chunks it needs.
import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import ComingSoon from './components/ComingSoon'

const HomePage = lazy(() => import('./pages/home/HomePage'))
const BaziPage = lazy(() => import('./pages/bazi/BaziPage'))
const DreamPage = lazy(() => import('./pages/dream/DreamPage'))
const HuangliPage = lazy(() => import('./pages/huangli/HuangliPage'))

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Suspense fallback={<div className="py-24 text-center text-muted">…</div>}>
          <Routes>
            <Route path="/" element={<HomePage />} />
            {/* Each feature lands its own page as its stage ships.
                Until then they share the ComingSoon placeholder. */}
            <Route path="/bazi" element={<BaziPage />} />
            <Route path="/dream" element={<DreamPage />} />
            <Route path="/huangli" element={<HuangliPage />} />
            <Route path="/zodiac" element={<ComingSoon titleKey="nav.zodiac" />} />
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
