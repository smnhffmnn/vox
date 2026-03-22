// Thin wrapper around auto-generated Wails v3 bindings.
// Bindings are generated at build time in frontend/bindings/

import { Events } from "@wailsio/runtime"

// Types matching Go structs in app.go

export interface ConfigResponse {
  language: string
  output: string
  raw: boolean
  hotkey: string
  mode: string
  handsfree_timeout: number
  doubletap_window: number
  notifications: boolean
  audio_feedback: boolean
  show_overlay: boolean
  stt_backend: string
  stt_url: string
  llm_backend: string
  llm_url: string
  llm_model: string
}

export interface StatusResponse {
  state: string
  uptime: string
  version: string
  has_key: boolean
}

export interface Snippet {
  trigger: string
  text: string
}

export interface HistoryEntry {
  timestamp: string
  language: string
  raw_text: string
  cleaned_text: string
  app_context: string
  duration_seconds: number
  backend: string
}

export interface TestResult {
  ok: boolean
  status: number
  error?: string
  message?: string
}

export interface PermissionStatus {
  accessibility: boolean
  microphone: boolean
}

// Re-export binding functions — import path will be resolved after wails3 generate bindings
// The actual module path depends on go module name: github.com/smnhffmnn/vox
export {
  GetConfig,
  SaveConfig,
  GetStatus,
  GetDictionary,
  SaveDictionary,
  GetSnippets,
  SaveSnippets,
  GetHistory,
  TestSTT,
  TestLLM,
  SetAPIKey,
  DeleteAPIKey,
  HasAPIKey,
  GetVersion,
  ShowWindow,
  GetPermissions,
  OpenAccessibilitySettings,
  OpenMicrophoneSettings,
} from "../../bindings/github.com/smnhffmnn/vox/app"

// State change event payload (emitted by Go backend)
export interface StateChangedEvent {
  state: string
  started_at?: number
}

// Events helper using Wails v3 runtime
export function EventsOn<T = unknown>(event: string, callback: (data: T) => void): () => void {
  const cancel = Events.On(event, (ev: { data: T }) => {
    callback(ev.data)
  })
  return () => cancel()
}
