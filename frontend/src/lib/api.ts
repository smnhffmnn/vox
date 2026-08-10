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

/**
 * Mirrors the Go constants in internal/history: "pending" means the recording
 * is stored but the transcription had not finished.
 */
export type EntryStatus = 'ok' | 'failed' | 'pending'

export interface HistoryEntry {
  id: string
  timestamp: string
  language: string
  raw_text: string
  cleaned_text: string
  app_context: string
  duration_seconds: number
  backend: string
  status: EntryStatus
  failed_step: string
  error_message: string
  /**
   * The transcript matched a known Whisper-hallucination pattern. It was
   * delivered and stored anyway — this is a hint to double-check the text.
   */
  suspected_hallucination: boolean
  /** Recording still on disk — audio is kept for the newest entries only. */
  has_audio: boolean
}

export interface HistoryInfo {
  path: string
  text_kept: number
  audio_kept: number
  audio_files: number
  audio_bytes: number
  /** Set when the recordings directory could not be read; counts are then meaningless. */
  usage_error?: string
}

export interface RetryResult {
  /** The transcription succeeded. Says nothing about delivery or saving. */
  ok: boolean
  /** The text reached the clipboard. */
  delivered: boolean
  /** The updated entry reached the history file. */
  persisted: boolean
  error?: string
  delivery_error?: string
  entry: HistoryEntry
}

export interface AudioData {
  base64: string
  mime_type: string
  filename: string
  error?: string
}

export interface LogRecord {
  time: string
  level: string
  step: string
  message: string
}

/** Emitted when a dictation attempt failed at any pipeline step. */
export interface AttemptFailedEvent {
  entry_id: string
  step: string
  message: string
  text: string
  can_retry: boolean
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
  GetHistoryInfo,
  RetryEntry,
  GetEntryAudio,
  RevealEntryAudio,
  RevealHistoryFile,
  CopyToClipboard,
  GetLogs,
  ClearLogs,
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

/** Emitted after a dictation was delivered. */
export interface TranscriptionEvent {
  raw: string
  cleaned: string
  /** The transcript matched a hallucination pattern — worth a second look. */
  suspected: boolean
}

// Events helper using Wails v3 runtime
export function EventsOn<T = unknown>(event: string, callback: (data: T) => void): () => void {
  const cancel = Events.On(event, (ev: { data: T }) => {
    callback(ev.data)
  })
  return () => cancel()
}
