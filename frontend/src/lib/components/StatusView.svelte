<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { status, appState } from '../stores'
  import { GetStatus, EventsOn, GetPermissions, OpenAccessibilitySettings, OpenMicrophoneSettings } from '../api'

  let lastTranscription = $state('')
  let accessibilityOk = $state(false)
  let microphoneOk = $state(false)
  let cleanupTranscriptionEvent: (() => void) | undefined
  let pollInterval: ReturnType<typeof setInterval>

  function stateLabel(state: string): string {
    switch (state) {
      case 'recording': return 'Recording'
      case 'processing': return 'Processing'
      default: return 'Idle'
    }
  }

  function stateColor(state: string): string {
    switch (state) {
      case 'recording': return 'var(--red)'
      case 'processing': return 'var(--orange)'
      default: return 'var(--text-muted)'
    }
  }

  async function refreshPermissions() {
    try {
      const p = await GetPermissions()
      accessibilityOk = p.accessibility
      microphoneOk = p.microphone
    } catch (e) {
      console.error('Failed to check permissions:', e)
    }
  }

  onMount(() => {
    refreshPermissions()

    pollInterval = setInterval(async () => {
      try {
        const s = await GetStatus()
        $status = s
        $appState = s.state
      } catch (e) {
        console.error('Failed to poll status:', e)
      }
      refreshPermissions()
    }, 2000)

    cleanupTranscriptionEvent = EventsOn('transcription', (data: { raw: string; cleaned: string }) => {
      lastTranscription = data.cleaned || data.raw
    })
  })

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval)
    if (cleanupTranscriptionEvent) cleanupTranscriptionEvent()
  })
</script>

<div class="section">
  <div class="section-header">
    <h2>Status</h2>
  </div>

  <div class="state-card">
    <div class="state-indicator">
      <div
        class="state-circle"
        class:pulse={$appState === 'recording' || $appState === 'processing'}
        style:background={stateColor($appState)}
        style:box-shadow="0 0 20px {stateColor($appState)}40"
      ></div>
      <span class="state-label">{stateLabel($appState)}</span>
    </div>
  </div>

  <div class="info-grid">
    <div class="info-card">
      <span class="info-label">Uptime</span>
      <span class="info-value">{$status?.uptime ?? '--'}</span>
    </div>
    <div class="info-card">
      <span class="info-label">Version</span>
      <span class="info-value">{$status?.version ?? '--'}</span>
    </div>
    <div class="info-card">
      <span class="info-label">API Key</span>
      <span class="info-value" class:configured={$status?.has_key} class:missing={!$status?.has_key}>
        {$status?.has_key ? 'Configured' : 'Not configured'}
      </span>
    </div>
  </div>

  <!-- Permissions -->
  <div class="permissions-section">
    <h3 class="permissions-title">Permissions</h3>
    <div class="permission-row">
      <div class="permission-info">
        <span class="permission-dot" class:ok={accessibilityOk} class:missing={!accessibilityOk}></span>
        <div>
          <span class="permission-name">Accessibility</span>
          <span class="permission-desc">Required for global hotkey and text injection</span>
        </div>
      </div>
      {#if !accessibilityOk}
        <button class="btn-sm" onclick={() => OpenAccessibilitySettings()}>Grant</button>
      {/if}
    </div>
    <div class="permission-row">
      <div class="permission-info">
        <span class="permission-dot" class:ok={microphoneOk} class:missing={!microphoneOk}></span>
        <div>
          <span class="permission-name">Microphone</span>
          <span class="permission-desc">Required for audio recording</span>
        </div>
      </div>
      {#if !microphoneOk}
        <button class="btn-sm" onclick={() => OpenMicrophoneSettings()}>Grant</button>
      {/if}
    </div>
  </div>

  {#if lastTranscription}
    <div class="transcription-card">
      <span class="info-label">Last Transcription</span>
      <p class="transcription-text">{lastTranscription}</p>
    </div>
  {/if}
</div>

<style>
  .section {
    margin-top: 4px;
  }

  .section-header {
    margin-bottom: 16px;
  }

  .section-header h2 {
    font-size: 15px;
    font-weight: 600;
  }

  .state-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 40px;
    display: flex;
    justify-content: center;
    margin-bottom: 16px;
  }

  .state-indicator {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
  }

  .state-circle {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    transition: background 0.3s ease, box-shadow 0.3s ease;
  }

  .state-circle.pulse {
    animation: pulse-circle 1.5s ease-in-out infinite;
  }

  @keyframes pulse-circle {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.6; transform: scale(0.95); }
  }

  .state-label {
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
  }

  .info-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
    margin-bottom: 16px;
  }

  .info-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .info-label {
    font-size: 11px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-muted);
  }

  .info-value {
    font-size: 14px;
    font-weight: 500;
    font-family: var(--font-mono);
  }

  .info-value.configured {
    color: var(--green);
  }

  .info-value.missing {
    color: var(--red);
  }

  .permissions-section {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px;
    margin-bottom: 16px;
  }

  .permissions-title {
    font-size: 11px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-muted);
    margin-bottom: 12px;
  }

  .permission-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;
  }

  .permission-row + .permission-row {
    border-top: 1px solid var(--border);
  }

  .permission-info {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .permission-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .permission-dot.ok {
    background: var(--green);
  }

  .permission-dot.missing {
    background: var(--red);
  }

  .permission-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--text);
    display: block;
  }

  .permission-desc {
    font-size: 11px;
    color: var(--text-muted);
    display: block;
  }

  .btn-sm {
    padding: 4px 12px;
    font-size: 12px;
    font-weight: 500;
    background: var(--accent);
    color: var(--btn-text, #fff);
    border: none;
    border-radius: var(--radius);
    cursor: pointer;
    transition: opacity 0.15s ease;
  }

  .btn-sm:hover {
    opacity: 0.85;
  }

  .transcription-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .transcription-text {
    font-size: 14px;
    line-height: 1.5;
    color: var(--text);
  }
</style>
