import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './i18n' // side-effect: configures react-i18next before App renders
import './index.css'
import { getTheme } from './theme'
import App from './App.tsx'

// Apply persisted theme before first render to avoid a flash.
document.documentElement.setAttribute('data-theme', getTheme())

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
