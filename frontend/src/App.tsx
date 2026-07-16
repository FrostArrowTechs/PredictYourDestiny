// Top-level router. Feature pages are lazy-loaded so the initial
// bundle stays small and each stage only adds the chunks it needs.
import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import { AuthProvider } from './auth/AuthContext'

const HomePage = lazy(() => import('./pages/home/HomePage'))
const BaziPage = lazy(() => import('./pages/bazi/BaziPage'))
const DreamPage = lazy(() => import('./pages/dream/DreamPage'))
const HuangliPage = lazy(() => import('./pages/huangli/HuangliPage'))
const ZodiacPage = lazy(() => import('./pages/zodiac/ZodiacPage'))
const CompatibilityPage = lazy(() => import('./pages/compatibility/CompatibilityPage'))
const WeighbonePage = lazy(() => import('./pages/weighbone/WeighbonePage'))
const DivinationPage = lazy(() => import('./pages/divination/DivinationPage'))
const PlumFlowerPage = lazy(() => import('./pages/plumflower/PlumFlowerPage'))
const NamePage = lazy(() => import('./pages/name/NamePage'))
const AstrologyPage = lazy(() => import('./pages/astrology/AstrologyPage'))
const ConstellationPage = lazy(() => import('./pages/constellation/ConstellationPage'))
const TarotPage = lazy(() => import('./pages/tarot/TarotPage'))
const ZiweiPage = lazy(() => import('./pages/ziwei/ZiweiPage'))
const LoginPage = lazy(() => import('./pages/auth/LoginPage'))
const RegisterPage = lazy(() => import('./pages/auth/RegisterPage'))
const AccountPage = lazy(() => import('./pages/account/AccountPage'))

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Layout>
          <Suspense fallback={<div className="py-24 text-center text-muted">…</div>}>
            <Routes>
              <Route path="/" element={<HomePage />} />
              {/* Each feature lands its own page as its stage ships.
                  Until then they share the ComingSoon placeholder. */}
              <Route path="/bazi" element={<BaziPage />} />
              <Route path="/dream" element={<DreamPage />} />
              <Route path="/huangli" element={<HuangliPage />} />
              <Route path="/zodiac" element={<ZodiacPage />} />
              <Route path="/compatibility" element={<CompatibilityPage />} />
              <Route path="/weighbone" element={<WeighbonePage />} />
              <Route path="/divination" element={<DivinationPage />} />
              <Route path="/plumflower" element={<PlumFlowerPage />} />
              <Route path="/name" element={<NamePage />} />
              <Route path="/astrology" element={<AstrologyPage />} />
              <Route path="/constellation" element={<ConstellationPage />} />
              <Route path="/tarot" element={<TarotPage />} />
              <Route path="/ziwei" element={<ZiweiPage />} />
              {/* Auth routes */}
              <Route path="/login" element={<LoginPage />} />
              <Route path="/register" element={<RegisterPage />} />
              <Route path="/account" element={<AccountPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </Layout>
      </AuthProvider>
    </BrowserRouter>
  )
}
