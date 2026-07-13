// Thin fetch wrapper for the JSON API.
//
// The backend ships as a separate image and may live on a different
// origin than the SPA (e.g. SPA on Cloudflare Pages, API on a container
// host). The base URL therefore reads from VITE_API_BASE_URL at build
// time; when unset it falls back to the same-origin /api path, which
// is what the Vite dev proxy serves.
//
//   VITE_API_BASE_URL=""            → /api           (dev proxy)
//   VITE_API_BASE_URL="https://api.example.com"  → https://api.example.com/api
const SAME_ORIGIN_BASE = '/api'
const configured = import.meta.env.VITE_API_BASE_URL as string | undefined
export const API_BASE = configured ? `${configured.replace(/\/$/, '')}/api` : SAME_ORIGIN_BASE
const BASE = API_BASE

export class ApiError extends Error {
  status: number
  body: unknown
  constructor(status: number, message: string, body: unknown) {
    super(message)
    this.status = status
    this.body = body
  }
}

interface Options extends Omit<RequestInit, 'body'> {
  body?: unknown
}

async function request<T>(path: string, opts: Options = {}): Promise<T> {
  const { body, headers, ...rest } = opts
  const res = await fetch(`${BASE}${path}`, {
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    ...rest,
  })

  // 204 or empty bodies parse to null safely.
  const text = await res.text()
  const parsed = text ? JSON.parse(text) : null

  if (!res.ok) {
    const msg =
      (parsed && typeof parsed === 'object' && 'error' in parsed && String((parsed as Record<string, unknown>).error)) ||
      res.statusText ||
      'request failed'
    throw new ApiError(res.status, msg, parsed)
  }
  return parsed as T
}

export const api = {
  get: <T>(path: string, opts?: Options) => request<T>(path, { ...opts, method: 'GET' }),
  post: <T>(path: string, body?: unknown, opts?: Options) =>
    request<T>(path, { ...opts, method: 'POST', body }),
  put: <T>(path: string, body?: unknown, opts?: Options) =>
    request<T>(path, { ...opts, method: 'PUT', body }),
  del: <T>(path: string, opts?: Options) => request<T>(path, { ...opts, method: 'DELETE' }),
}

// ── typed endpoints ───────────────────────────────────────────────
export interface HealthResponse {
  status: string
  version: string
  time: string
}

export const Health = {
  check: () => api.get<HealthResponse>('/health'),
}
