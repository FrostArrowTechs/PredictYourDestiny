/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Cross-origin API origin, e.g. https://api.example.com. Empty/undefined → same-origin /api. */
  readonly VITE_API_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
