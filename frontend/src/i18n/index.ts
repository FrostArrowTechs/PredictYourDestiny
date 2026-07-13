// i18n bootstrap.
//
// react-i18next is configured with two Chinese variants for MVP.
// Adding a language is a three-step affair: create a locale file,
// register it in `resources` below, and add its code to the language
// switcher in components/LanguageSwitcher.tsx.
//
// The browser's preferred language is detected via navigator, but
// the user's explicit choice is persisted to localStorage so it
// survives reloads and overrides any OS setting.
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import zhCN, { type Translation } from './locales/zh-CN'
import zhTW from './locales/zh-TW'

export const SUPPORTED_LANGS = ['zh-CN', 'zh-TW'] as const
export type Lang = (typeof SUPPORTED_LANGS)[number]
export const DEFAULT_LANG: Lang = 'zh-CN'

const STORAGE_KEY = 'pyd.lang'

/** Read the persisted choice, falling back to the browser language. */
function detectInitialLang(): Lang {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved && (SUPPORTED_LANGS as readonly string[]).includes(saved)) {
    return saved as Lang
  }
  const nav = navigator.language
  if (nav.startsWith('zh-TW') || nav.startsWith('zh-Hant')) return 'zh-TW'
  return DEFAULT_LANG
}

export function rememberLang(lang: Lang) {
  localStorage.setItem(STORAGE_KEY, lang)
}

void i18n.use(initReactI18next).init({
  resources: {
    'zh-CN': { translation: zhCN },
    'zh-TW': { translation: zhTW },
  },
  lng: detectInitialLang(),
  fallbackLng: DEFAULT_LANG,
  interpolation: { escapeValue: false }, // React already escapes
  returnObjects: false,
})

// Keep ts happy about the typed resources we feed in.
export type { Translation }

export default i18n
