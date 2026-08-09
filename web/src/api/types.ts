export type ResourceStatus = 'unknown' | 'healthy' | 'unhealthy' | 'cooldown'

export interface SessionResponse {
  authenticated: true
  csrf_token: string
  expires_at: number
}

export interface APIKeyState {
  hint: string
  external: boolean
}

export interface Account {
  id: number
  label: string
  cookie_summary: string
  enabled: boolean
  status: ResourceStatus
  note: string
  proxy_id: number
  failure_count: number
  last_used_at: number
  last_success_at: number
  last_error: string
  created_at: number
  updated_at: number
}

export interface Proxy {
  id: number
  label: string
  url_summary: string
  enabled: boolean
  status: ResourceStatus
  failure_count: number
  last_used_at: number
  last_success_at: number
  last_error: string
  cooldown_until: number
  created_at: number
  updated_at: number
}

export interface RequestRow {
  id: number
  request_id: string
  created_at: number
  endpoint: string
  model: string
  upstream_model: string
  status_code: number
  latency_ms: number
  ttfb_ms: number
  input_tokens: number
  output_tokens: number
  stream: boolean
  account_id: number
  account_label: string
  proxy_id: number
  proxy_label: string
  error_code: string
  error_message: string
}

export interface RequestPage {
  items: RequestRow[]
  total: number
  limit: number
  offset: number
}

export interface OverviewStats {
  requests: number
  success_rate: number | null
  p50_latency_ms: number | null
  output_tokens: number
  accounts: number
  healthy_accounts: number
  proxies: number
}

export interface TimePoint {
  bucket: number
  requests: number
  failures: number
  latency_ms: number
}

export interface OverviewResponse {
  stats: OverviewStats
  timeseries: TimePoint[]
  recent: RequestRow[]
  accounts: Account[]
  api_key: APIKeyState
  range_hours: number
}

export interface RuntimeSettings {
  default_model: string
  request_timeout_sec: number
  retry_attempts: number
  retry_delay_ms: number
  max_prompt_bytes: number
  fallback_anonymous: boolean
  fallback_direct: boolean
  gemini_bl: string
  gemini_bl_auto: boolean
  retention_days: number
}

export interface SettingsResponse {
  settings: RuntimeSettings
  available_models: string[]
  password_source: string
}

export interface ProbeResult {
  ok: boolean
  model: string
  upstream_model?: string
  latency_ms: number
  ttfb_ms?: number
  error?: string
}

export interface APIKeyRotation {
  key: string
  hint: string
  notice: string
}
