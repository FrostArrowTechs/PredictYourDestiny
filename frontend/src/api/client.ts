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

// ── bazi (四柱命理) ──────────────────────────────────────────────
//
// These mirror the JSON shapes the Go handler returns verbatim (see
// backend/internal/fortune/bazi.go), so a backend field rename shows
// up as a TS compile error here.

export interface Pillar {
  position: string // 年|月|日|时
  ganZhi: string // e.g. 甲子
  gan: string // 天干
  zhi: string // 地支
  wuXing: string // e.g. 木水 (gan + zhi element)
  naYin: string // 纳音
  hideGan: string[] // 地支藏干
  shiShenGan: string // 天干十神
  shiShenZhi: string[] // 地支藏干十神
  diShi: string // 十二长生
  xun: string
  xunKong: string
}

export interface ShenSha {
  name: string
  position: string
  ganZhi: string
  note: string
}

export interface LiuNian {
  year: number
  age: number
  ganZhi: string
}

export interface DaYun {
  index: number // 0 = 起运前
  ganZhi: string
  startYear: number
  endYear: number
  startAge: number
  endAge: number
  liuNian: LiuNian[]
}

export interface WuXingStat {
  element: string // 金|木|水|火|土
  count: number
  percent: number
}

export interface WangShuai {
  strong: string
  weak: string
  dayWang: string // 偏旺|偏弱|平衡
  summary: string
}

export interface YongYin {
  yongShen: string
  xi: string[]
  ji: string[]
  confidence: string
  reason: string
}

export interface BaziChart {
  solar: string
  lunar: string
  solarISO: string
  longitude: number
  trueSolar: boolean
  correction: string
  pillars: Pillar[]
  taiYuan: string
  taiYuanNaYin: string
  taiXi: string
  taiXiNaYin: string
  mingGong: string
  mingGongNaYin: string
  shenGong: string
  shenGongNaYin: string
  shenSha: ShenSha[]
  startYear: number
  startMonth: number
  startDay: number
  startHour: number
  forward: boolean
  startSolar: string
  daYun: DaYun[]
  wuXingStats: WuXingStat[]
  wangShuai: WangShuai
  yongYin: YongYin
}

export interface BaziResult {
  kind: 'bazi'
  data: BaziChart
  meta: Record<string, string>
}

export interface BaziInput {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  gender: 0 | 1 // 0 女, 1 男
  longitude?: number
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export interface BaziInterpretResponse {
  content: string
  reasoning: string
  model: string
  usage: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
    reasoning_tokens?: number
  }
}

export const Bazi = {
  compute: (input: BaziInput) => api.post<BaziResult>('/bazi/compute', input),
  interpret: (input: BaziInput) => api.post<BaziInterpretResponse>('/bazi/interpret', input),
}

// ── model catalog (for tier-aware model picker UI) ───────────────

export interface ModelEntry {
  id: string
  label: string
  tier: 'free' | 'paid'
}

export interface ModelCatalog {
  free: ModelEntry[]
  paid: ModelEntry[]
}

// ── SSE streaming for /bazi/interpret ────────────────────────────
//
// The sync `api.post` helper JSON-parses the whole body, which breaks
// for SSE. streamBaziInterpret opens a fetch with stream=true, reads
// the response as a ReadableStream, and emits each `data: {...}` line
// as a parsed event. The caller renders deltas as they arrive for the
// typewriter effect.

export interface InterpretStreamEvent {
  content?: string
  reasoning?: string
  usage?: BaziInterpretResponse['usage']
  done?: boolean
  error?: string
}

export async function streamBaziInterpret(
  input: BaziInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${API_BASE}/bazi/interpret`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ ...input, stream: true }),
    signal,
  })

  if (!res.ok) {
    const text = await res.text().catch(() => '')
    let msg = res.statusText
    try {
      const j = JSON.parse(text) as { error?: string }
      if (j.error) msg = j.error
    } catch {
      if (text) msg = text
    }
    onEvent({ error: msg })
    return
  }

  if (!res.body) {
    onEvent({ error: 'empty stream' })
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    // SSE events are separated by \n\n; handle multiple per chunk.
    let idx: number
    while ((idx = buffer.indexOf('\n\n')) !== -1) {
      const rawEvent = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      const line = parseSseDataLine(rawEvent)
      if (line === null) continue
      if (line === '[DONE]') {
        onEvent({ done: true })
        return
      }
      try {
        onEvent(JSON.parse(line) as InterpretStreamEvent)
      } catch {
        // skip malformed event
      }
    }
  }
  onEvent({ done: true })
}

// parseSseDataLine pulls the payload out of one SSE event block
// (which may contain a "data:" line plus comments/ids). Returns null
// if the block has no data line.
function parseSseDataLine(block: string): string | null {
  for (const line of block.split('\n')) {
    const trimmed = line.trimStart()
    if (trimmed.startsWith('data:')) {
      return trimmed.slice(5).trimStart()
    }
  }
  return null
}

// ── dream (周公解梦) ──────────────────────────────────────────────

export interface DreamMatch {
  keyword: string
  category: string
  meaning: string
}

export interface DreamChart {
  question: string
  matches: DreamMatch[]
  totalMatches: number
}

export interface DreamResult {
  kind: 'dream'
  data: DreamChart
  meta: Record<string, string>
}

export interface DreamInput {
  question: string
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export interface DreamInterpretResponse {
  content: string
  reasoning: string
  model: string
  usage: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
    reasoning_tokens?: number
  }
}

export const Dream = {
  compute: (input: DreamInput) => api.post<DreamResult>('/dream/compute', input),
  interpret: (input: DreamInput) => api.post<DreamInterpretResponse>('/dream/interpret', input),
}

// SSE streaming for /dream/interpret
export async function streamDreamInterpret(
  input: DreamInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${API_BASE}/dream/interpret`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ ...input, stream: true }),
    signal,
  })

  if (!res.ok) {
    const text = await res.text().catch(() => '')
    let msg = res.statusText
    try {
      const j = JSON.parse(text) as { error?: string }
      if (j.error) msg = j.error
    } catch {
      if (text) msg = text
    }
    onEvent({ error: msg })
    return
  }

  if (!res.body) {
    onEvent({ error: 'empty stream' })
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let idx: number
    while ((idx = buffer.indexOf('\n\n')) !== -1) {
      const rawEvent = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      const line = parseSseDataLine(rawEvent)
      if (line === null) continue
      if (line === '[DONE]') {
        onEvent({ done: true })
        return
      }
      try {
        onEvent(JSON.parse(line) as InterpretStreamEvent)
      } catch {
        // skip malformed event
      }
    }
  }
  onEvent({ done: true })
}

// ── huangli (万年历黄历) ──────────────────────────────────────────────

export interface HuangliChart {
  solar: string
  lunar: string
  yearGanZhi: string
  monthGanZhi: string
  dayGanZhi: string
  yi: string[]
  ji: string[]
  jiShen: string[]
  xiongSha: string[]
  pengZu: string
  chong: string
  sha: string
  wuXing: string
  naYin: string
  xingZuo: string
  erShiBaXiu: string
  yueXiang: string
  jieQi: string
  week: string
  taiShen: string
}

export interface HuangliResult {
  kind: 'huangli'
  data: HuangliChart
  meta: Record<string, string>
}

export interface HuangliInput {
  year: number
  month: number
  day: number
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
  activity?: string
}

export interface HuangliInterpretResponse {
  content: string
  reasoning: string
  model: string
  usage: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
    reasoning_tokens?: number
  }
}

export const Huangli = {
  compute: (input: HuangliInput) => api.post<HuangliResult>('/huangli/compute', input),
  interpret: (input: HuangliInput) => api.post<HuangliInterpretResponse>('/huangli/interpret', input),
}

// SSE streaming for /huangli/interpret
export async function streamHuangliInterpret(
  input: HuangliInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${API_BASE}/huangli/interpret`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ ...input, stream: true }),
    signal,
  })

  if (!res.ok) {
    const text = await res.text().catch(() => '')
    let msg = res.statusText
    try {
      const j = JSON.parse(text) as { error?: string }
      if (j.error) msg = j.error
    } catch {
      if (text) msg = text
    }
    onEvent({ error: msg })
    return
  }

  if (!res.body) {
    onEvent({ error: 'empty stream' })
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let idx: number
    while ((idx = buffer.indexOf('\n\n')) !== -1) {
      const rawEvent = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      const line = parseSseDataLine(rawEvent)
      if (line === null) continue
      if (line === '[DONE]') {
        onEvent({ done: true })
        return
      }
      try {
        onEvent(JSON.parse(line) as InterpretStreamEvent)
      } catch {
        // skip malformed event
      }
    }
  }
  onEvent({ done: true })
}

// ── zodiac (生肖运势) ──────────────────────────────────────────────

export interface ZodiacRelation {
  type: string
  with: string
  effect: string
}

export interface ZodiacChart {
  zodiac: string
  year: number
  liuNianZhi: string
  liuNianZodiac: string
  overallScore: number
  careerScore: number
  wealthScore: number
  loveScore: number
  healthScore: number
  relations: ZodiacRelation[]
  luckyColors: string[]
  luckyNumbers: number[]
  luckyDir: string
  tips: string[]
  warns: string[]
}

export interface ZodiacResult {
  kind: 'zodiac'
  data: ZodiacChart
  meta: Record<string, string>
}

export interface ZodiacInput {
  year: number
  month?: number
  day?: number
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Zodiac = {
  compute: (input: ZodiacInput) => api.post<ZodiacResult>('/zodiac/compute', input),
  interpret: (input: ZodiacInput) => api.post<BaziInterpretResponse>('/zodiac/interpret', input),
}

export async function streamZodiacInterpret(
  input: ZodiacInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${API_BASE}/zodiac/interpret`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ ...input, stream: true }),
    signal,
  })
  await consumeSSE(res, onEvent)
}

// ── compatibility (男女配对) ───────────────────────────────────────

export interface CompatibilitySubject {
  zodiac: string
  yearGanZhi: string
  dayGan: string
  dayZhi: string
  dayWuXing: string
}

export interface CompatibilityFactor {
  factor: string
  score: number
  detail: string
}

export interface CompatibilityChart {
  subject1: CompatibilitySubject
  subject2: CompatibilitySubject
  overallScore: number
  chemistryScore: number
  harmonyScore: number
  stabilityScore: number
  factors: CompatibilityFactor[]
  summary: string
  tips: string
}

export interface CompatibilityResult {
  kind: 'compatibility'
  data: CompatibilityChart
  meta: Record<string, string>
}

export interface SubjectInput {
  year: number
  month: number
  day: number
}

export interface CompatibilityInput {
  year: number
  month: number
  day: number
  second: SubjectInput
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Compatibility = {
  compute: (input: CompatibilityInput) => api.post<CompatibilityResult>('/compatibility/compute', input),
  interpret: (input: CompatibilityInput) => api.post<BaziInterpretResponse>('/compatibility/interpret', input),
}

export async function streamCompatibilityInterpret(
  input: CompatibilityInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${API_BASE}/compatibility/interpret`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ ...input, stream: true }),
    signal,
  })
  await consumeSSE(res, onEvent)
}

// ── shared SSE consumer helper ─────────────────────────────────────

async function consumeSSE(
  res: Response,
  onEvent: (ev: InterpretStreamEvent) => void,
): Promise<void> {
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    let msg = res.statusText
    try {
      const j = JSON.parse(text) as { error?: string }
      if (j.error) msg = j.error
    } catch {
      if (text) msg = text
    }
    onEvent({ error: msg })
    return
  }

  if (!res.body) {
    onEvent({ error: 'empty stream' })
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let idx: number
    while ((idx = buffer.indexOf('\n\n')) !== -1) {
      const rawEvent = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      const line = parseSseDataLine(rawEvent)
      if (line === null) continue
      if (line === '[DONE]') {
        onEvent({ done: true })
        return
      }
      try {
        onEvent(JSON.parse(line) as InterpretStreamEvent)
      } catch {
        // skip malformed event
      }
    }
  }
  onEvent({ done: true })
}
