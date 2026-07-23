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

function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function invalidateSession(): void {
  localStorage.removeItem('pyd_token')
  window.dispatchEvent(new Event('pyd:unauthorized'))
}

function streamHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'text/event-stream',
  }
  const token = localStorage.getItem('pyd_token')
  if (token) headers.Authorization = `Bearer ${token}`
  headers['Idempotency-Key'] = newIdempotencyKey()
  headers['X-Request-ID'] = newIdempotencyKey()
  return headers
}

async function streamInterpret<T extends object>(
  path: string,
  input: T,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  try {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'POST',
      headers: streamHeaders(),
      body: JSON.stringify({ ...input, stream: true }),
      signal,
    })
    await consumeSSE(res, onEvent)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    onEvent({ error: error instanceof Error ? error.message : 'network error' })
  }
}

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

  // Auto-attach JWT token if available
  const token = localStorage.getItem('pyd_token')
  const baseHeaders: Record<string, string> = {
    Accept: 'application/json',
    'X-Request-ID': newIdempotencyKey(),
  }
  if (body !== undefined) baseHeaders['Content-Type'] = 'application/json'
  if (token) baseHeaders.Authorization = `Bearer ${token}`
  if (rest.method === 'POST' && path.endsWith('/interpret')) {
    baseHeaders['Idempotency-Key'] = newIdempotencyKey()
  }

  const res = await fetch(`${BASE}${path}`, {
    headers: {
      ...baseHeaders,
      ...(headers as Record<string, string> | undefined),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    ...rest,
  })

  if (res.status === 401) invalidateSession()

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
export interface ResultMetadata {
  algorithmVersion: string
  ruleSetVersion: string
  inputPrecision: 'minute' | 'hour' | 'period' | 'shichen' | 'unknown'
  assumptions: string[]
  warnings: string[]
  lunarMonthWasLeap: boolean
  leapMonthRule?: string
  effectiveLunarMonth: number
  unsupportedRules: string[]
  stableFacts: Array<{ key: string; value: unknown }>
  variableFacts: Array<{ key: string; value: unknown }>
  variants: Array<{ fingerprint: string; label?: string; data?: unknown }>
  evidence: Array<{ source: string; description: string }>
}

export interface HealthResponse {
  status: string
  version: string
  time: string
}

export const Health = {
  check: () => api.get<HealthResponse>('/health'),
}

export interface EntitlementResponse {
  effectiveTier: string
  tierName: string
  expiresAt: string | null
  dailyQuota: number
  features: string[]
  availableModels: ModelEntry[]
  fellBackToFree: boolean
}

export const Entitlements = {
  get: () => api.get<EntitlementResponse>('/entitlements'),
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

export interface BaziInterpretation {
  ruleSetVersion: string
  nature: 'interpretive_heuristic'
  inputFacts: Array<{ key: string; value: unknown }>
  warnings: string[]
  wangShuai: WangShuai
  yongYin: YongYin
}

export interface BaziChart {
  solar: string
  lunar: string
  solarISO: string
  longitude: number
  trueSolar: boolean
  correction: string
  timeZone: string
  solarTimeMode: 'legal_time' | 'local_apparent_solar'
  longitudeCorrectionMinutes: number
  equationOfTimeMinutes: number
  totalCorrectionMinutes: number
  ruleSetVersion: string
  dayBoundary: 'midnight' | 'zi_chu_23:00'
  calendarLibraryVersion: string
  previousJie: { name: string; time: string }
  nextJie: { name: string; time: string }
  yunMethod: string
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
  interpretation: BaziInterpretation
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
  timeZone?: string
  ruleSet?: 'bazi-standard-v1' | 'bazi-zi-chu-v1'
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
  return streamInterpret('/bazi/interpret', input, onEvent, signal)
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
  return streamInterpret('/dream/interpret', input, onEvent, signal)
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
  return streamInterpret('/huangli/interpret', input, onEvent, signal)
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
  return streamInterpret('/zodiac/interpret', input, onEvent, signal)
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
  return streamInterpret('/compatibility/interpret', input, onEvent, signal)
}

// ── shared SSE consumer helper ─────────────────────────────────────

async function consumeSSE(
  res: Response,
  onEvent: (ev: InterpretStreamEvent) => void,
): Promise<void> {
  if (res.status === 401) invalidateSession()
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

// ── weighbone (称骨算命) ───────────────────────────────────────────

export interface WeighboneChart {
  yearWeight: string
  monthWeight: string
  dayWeight: string
  hourWeight: string
  totalWeight: string
  totalQian: number
  poem: string
  category: string
  description: string
}

export interface WeighboneResult {
  kind: 'weighbone'
  data: WeighboneChart
  meta: Record<string, string>
}

export interface WeighboneInput {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Weighbone = {
  compute: (input: WeighboneInput) => api.post<WeighboneResult>('/weighbone/compute', input),
}

export async function streamWeighboneInterpret(
  input: WeighboneInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/weighbone/interpret', input, onEvent, signal)
}

// ── divination (抽签/求签) ─────────────────────────────────────────

export interface DivinationChart {
  number: number
  title: string
  tier: string
  poem: string
  interpret: string
  category: string
  question: string
}

export interface DivinationResult {
  kind: 'divination'
  data: DivinationChart
  meta: Record<string, string>
}

export interface DivinationInput {
  question?: string
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Divination = {
  compute: (input: DivinationInput) => api.post<DivinationResult>('/divination/compute', input),
}

export async function streamDivinationInterpret(
  input: DivinationInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/divination/interpret', input, onEvent, signal)
}

// ── plumflower (梅花易数) ──────────────────────────────────────────

export interface Hexagram {
  name: string
  upperTrig: string
  lowerTrig: string
  upperWX: string
  lowerWX: string
  lines: [number, number, number, number, number, number]
}

export interface PlumFlowerChart {
  method: string
  original: Hexagram
  mutual: Hexagram
  changed: Hexagram
  changingLine: number
  bodyTrigram: string
  useTrigram: string
  bodyWuXing: string
  useWuXing: string
  relationship: string
  trend: string
  analysis: string
}

export interface PlumFlowerResult {
  kind: 'plumflower'
  data: PlumFlowerChart
  meta: Record<string, string>
}

export interface PlumFlowerInput {
  year?: number
  month?: number
  day?: number
  hour?: number
  question?: string
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const PlumFlower = {
  compute: (input: PlumFlowerInput) => api.post<PlumFlowerResult>('/plumflower/compute', input),
}

export async function streamPlumFlowerInterpret(
  input: PlumFlowerInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/plumflower/interpret', input, onEvent, signal)
}

// ── name (姓名学) ─────────────────────────────────────────────────────

export interface StrokeDetail {
  char: string
  strokes: number
  wuXing: string
  position: string
  charIndex: number
}

export interface NameChart {
  fullName: string
  surname: string
  givenName: string
  surnameConfirmed: boolean
  inputMode: 'structured' | 'legacy_auto_split'
  script: 'zh-Hans' | 'zh-Hant'
  strokeStandard: string
  dictionaryVersion: string
  ruleSetVersion: string
  warnings: string[]
  tianGe: number
  renGe: number
  diGe: number
  waiGe: number
  zongGe: number
  sanCai: string
  tianGeLuck: string
  renGeLuck: string
  diGeLuck: string
  waiGeLuck: string
  zongGeLuck: string
  traditionalMatchScore: number
  traditionalMatchDesc: string
  evaluations: Array<{
    dimension: 'traditional_numerology' | 'pronunciation' | 'meaning' | 'writing_compatibility'
    status: 'available' | 'unavailable' | 'basic_check'
    score?: number
    summary: string
    evidence: string[]
    warnings: string[]
  }>
  strokeDetails: StrokeDetail[]
}

export interface NameResult {
  kind: 'name'
  data: NameChart
  meta: Record<string, string>
}

export interface NameInput {
  fullName?: string
  surname?: string
  givenName?: string
  surnameConfirmed?: boolean
  script?: 'zh-Hans' | 'zh-Hant'
  strokeStandard?: 'kangxi'
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Name = {
  compute: (input: NameInput) => api.post<NameResult>('/name/compute', input),
}

export async function streamNameInterpret(
  input: NameInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/name/interpret', input, onEvent, signal)
}

// ── astrology (占星本命盘) ──────────────────────────────────────────────

export interface PlanetInfo {
  name: string
  sign: string
  degree: number
  house: number
  retrograde: boolean
}

export interface HouseInfo {
  number: number
  sign: string
  degree: number
}

export interface AspectInfo {
  planet1: string
  planet2: string
  aspect: string
  orb: number
  exact: boolean
}

export interface AstrologyChart {
  accuracyLabel: string
  timeZone: string
  utcInstant: string
  sunSign: string
  moonSign: string
  ascendant: string
  planets: PlanetInfo[]
  houses: HouseInfo[]
  aspects: AspectInfo[]
  chartSummary: string
}

export interface AstrologyResult {
  // Current responses use the common {kind,data,meta,resultMetadata} envelope;
  // optional flat fields keep older cached responses readable.
  kind?: 'astrology'
  data?: AstrologyChart
  meta?: Record<string, string>
  resultMetadata?: ResultMetadata
  accuracyLabel?: string
  sunSign?: string
  moonSign?: string
  ascendant?: string
  planets?: PlanetInfo[]
  houses?: HouseInfo[]
  aspects?: AspectInfo[]
  chartSummary?: string
}

// Normalize whatever the backend returns into the flat chart shape
// the page actually consumes. Tolerates the historical {data: ...} wrapper
// in case it's ever reintroduced.
export function asAstrologyChart(r: AstrologyResult): AstrologyChart {
  if (r.data) {
    return {
      ...r.data,
      accuracyLabel: r.data.accuracyLabel ?? '娱乐性简化版',
      planets: r.data.planets ?? [],
      houses: r.data.houses ?? [],
      aspects: r.data.aspects ?? [],
    }
  }
  return {
    accuracyLabel: r.accuracyLabel ?? '娱乐性简化版',
    timeZone: '',
    utcInstant: '',
    sunSign: r.sunSign ?? '',
    moonSign: r.moonSign ?? '',
    ascendant: r.ascendant ?? '',
    planets: r.planets ?? [],
    houses: r.houses ?? [],
    aspects: r.aspects ?? [],
    chartSummary: r.chartSummary ?? '',
  }
}

export interface AstrologyInput {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  longitude?: number
  latitude?: number
  timeZone: string
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Astrology = {
  compute: (input: AstrologyInput) => api.post<AstrologyResult>('/astrology/compute', input),
}

export async function streamAstrologyInterpret(
  input: AstrologyInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/astrology/interpret', input, onEvent, signal)
}

// ── constellation (星座运势) ──────────────────────────────────────────

export interface ConstellationChart {
  sign: string
  signLatin: string
  element: string
  quality: string
  ruler: string
  dateRange: string
  strengths: string[]
  weakness: string[]
  keywords: string[]
  overallScore: number
  careerScore: number
  loveScore: number
  wealthScore: number
  healthScore: number
  luckyColors: string[]
  luckyNumbers: number[]
  luckyDir: string
  bestMatch: string
  worstMatch: string
}

export interface ConstellationResult {
  kind: 'constellation'
  data: ConstellationChart
  meta: Record<string, string>
}

export interface ConstellationInput {
  year: number
  month: number
  day: number
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Constellation = {
  compute: (input: ConstellationInput) => api.post<ConstellationResult>('/constellation/compute', input),
}

export async function streamConstellationInterpret(
  input: ConstellationInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/constellation/interpret', input, onEvent, signal)
}

// ── tarot (塔罗) ──────────────────────────────────────────────────────

export interface TarotSpread {
  id: string
  name: string
  count: number
  labels: string[]
}

export interface TarotCardDraw {
  number: number
  name: string
  nameLatin: string
  arcana: string
  suit: string
  reversed: boolean
  positionIndex: number
  positionLabel: string
  meaning: string
  keywords: string
  element: string
}

export interface TarotChart {
  spread: TarotSpread
  cards: TarotCardDraw[]
  question: string
}

export interface TarotResult {
  kind: 'tarot'
  data: TarotChart
  meta: Record<string, string>
}

export interface TarotInput {
  spread?: 'single' | 'three' | 'celtic'
  question?: string
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Tarot = {
  draw: (input: TarotInput) => api.post<TarotResult>('/tarot/draw', input),
}

export async function streamTarotInterpret(
  input: TarotInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/tarot/interpret', input, onEvent, signal)
}

// ── ziwei (紫微斗数) ──────────────────────────────────────────────────

export interface ZiweiPalace {
  branch: string
  position: number
  name: string
  isLife: boolean
  isBody: boolean
  stars: string[]
  transform: string
  transformations: ZiweiTransformation[]
}

export interface ZiweiTransformation {
  ruleSetVersion: string
  star: string
  label: string
  position: number
}

export interface ZiweiChart {
  algorithmVersion: string
  rulePack: {
    version: string
    status: 'provisional' | 'verified'
    calendarVersion: string
    supportedRules: string[]
    approximateRules: string[]
    unsupportedRules: string[]
    evidence: string[]
  }
  warnings: string[]
  solarDate: string
  lunarDate: string
  gender: string
  yearGanZhi: string
  monthGanZhi: string
  dayGanZhi: string
  lifePalaceBranch: string
  bodyPalaceBranch: string
  lifeRuler: string
  bodyRuler: string
  palaces: ZiweiPalace[]
  wuXingJu: string
  daYunStartAge: number
  daYunForward: boolean
  mainStarOfLife: string
  transformations: ZiweiTransformation[]
}

export interface ZiweiResult {
  kind: 'ziwei'
  data: ZiweiChart
  meta: Record<string, string>
}

export interface ZiweiInput {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  gender: 0 | 1
  ziweiLeapMonthRule?: '' | 'as_next_month-v1' | 'split_at_day_15-v1'
  lang?: string
  interpretDepth?: 'brief' | 'deep'
  model?: string
  stream?: boolean
}

export const Ziwei = {
  compute: (input: ZiweiInput) => api.post<ZiweiResult>('/ziwei/compute', input),
}

export async function streamZiweiInterpret(
  input: ZiweiInput,
  onEvent: (ev: InterpretStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  return streamInterpret('/ziwei/interpret', input, onEvent, signal)
}

// ── auth (认证) ────────────────────────────────────────────────────

export interface AuthUser {
  id: number
  email: string
  displayName: string
  role: string
}

export interface AuthResponse {
  token: string
  user: AuthUser
}

export interface LoginInput {
  email: string
  password: string
}

export interface RegisterInput {
  email: string
  password: string
  displayName?: string
}

export const Auth = {
  register: (input: RegisterInput) => api.post<AuthResponse>('/auth/register', input),
  login: (input: LoginInput) => api.post<AuthResponse>('/auth/login', input),
  me: () => api.get<{ user: AuthUser }>('/auth/me'),
}

// ── records (用户历史记录) ──────────────────────────────────────────

export interface FortuneRecord {
  id: number
  kind: string
  title: string
  inputJson: string
  resultJson: string
  note: string
  createdAt: string
}

export interface RecordsResponse {
  records: FortuneRecord[]
  total: number
  page: number
  limit: number
}

export interface CreateRecordInput {
  kind: string
  title?: string
  inputJson: string
  resultJson: string
  note?: string
}

export const Records = {
  list: (params?: { page?: number; limit?: number; kind?: string }) => {
    const query = new URLSearchParams()
    if (params?.page) query.set('page', String(params.page))
    if (params?.limit) query.set('limit', String(params.limit))
    if (params?.kind) query.set('kind', params.kind)
    const qs = query.toString()
    return api.get<RecordsResponse>(`/records${qs ? `?${qs}` : ''}`)
  },
  get: (id: number) => api.get<FortuneRecord>(`/records/${id}`),
  create: (input: CreateRecordInput) => api.post<FortuneRecord>('/records', input),
  delete: (id: number) => api.del<{ message: string }>(`/records/${id}`),
}

// ── quota (配额) ────────────────────────────────────────────────────

export interface QuotaResponse {
  date: string
  used: number
  remaining: number
  limit: number
}

export const Quota = {
  get: () => api.get<QuotaResponse>('/quota'),
}
