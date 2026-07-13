import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './i18n' // side-effect: configures react-i18next before App renders
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
