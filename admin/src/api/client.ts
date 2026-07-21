const API_BASE = (import.meta.env.VITE_ADMIN_API_BASE_URL || '/api').replace(/\/$/, '')
const TOKEN_KEY = 'admin_token'
const UNAUTHORIZED_EVENT = 'pyd:admin-unauthorized'

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export function getAdminToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function setAdminToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearAdminToken() {
  localStorage.removeItem(TOKEN_KEY)
}

interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
  auth?: boolean
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, auth = true, headers: suppliedHeaders, ...init } = options
  const headers = new Headers(suppliedHeaders)
  headers.set('Accept', 'application/json')
  headers.set('X-Request-ID', crypto.randomUUID())

  if (auth) {
    const token = getAdminToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
  }
  if (body !== undefined) headers.set('Content-Type', 'application/json')

  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...init,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new ApiError('Unable to reach the API', 0)
  }

  if (response.status === 401 && auth) {
    clearAdminToken()
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
  }

  const text = await response.text()
  let payload: unknown
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = text
    }
  }

  if (!response.ok) {
    const message = typeof payload === 'object' && payload !== null && 'error' in payload
      ? String((payload as { error: unknown }).error)
      : `Request failed (${response.status})`
    throw new ApiError(message, response.status)
  }

  return payload as T
}

export const adminUnauthorizedEvent = UNAUTHORIZED_EVENT
