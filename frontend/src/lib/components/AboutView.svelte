<script lang="ts">
  import { onMount } from 'svelte'
  import { GetVersion, GetStatus } from '../api'
  import type { StatusResponse } from '../api'

  let version = $state('')
  let statusInfo = $state<StatusResponse | null>(null)

  onMount(async () => {
    try {
      const [v, s] = await Promise.all([GetVersion(), GetStatus()])
      version = v
      statusInfo = s
    } catch (e) {
      console.error('Failed to load about info:', e)
    }
  })
</script>

<div class="section">
  <div class="section-header">
    <h2>About</h2>
  </div>

  <div class="about-hero">
    <div class="about-icon">
      <svg viewBox="0 0 48 48" fill="none" width="48" height="48">
        <circle cx="24" cy="24" r="22" stroke="var(--accent)" stroke-width="2" fill="none" />
        <rect x="20" y="10" width="8" height="16" rx="4" fill="var(--accent)" />
        <path d="M16 22v2a8 8 0 0016 0v-2" stroke="var(--accent)" stroke-width="2" fill="none" stroke-linecap="round" />
        <line x1="24" y1="32" x2="24" y2="38" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" />
        <line x1="19" y1="38" x2="29" y2="38" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" />
      </svg>
    </div>
    <div class="about-title">
      <h1>Vox</h1>
      {#if version}
        <span class="about-version">v{version}</span>
      {/if}
    </div>
    <p class="about-description">
      Speech-to-text dictation tool for macOS, Linux and Windows. Press a hotkey to record,
      release to transcribe and inject text into any application.
    </p>
  </div>

  <div class="about-card">
    <div class="about-row">
      <span class="about-label">Version</span>
      <span class="about-value">{version || '--'}</span>
    </div>
    <div class="about-row">
      <span class="about-label">Uptime</span>
      <span class="about-value">{statusInfo?.uptime ?? '--'}</span>
    </div>
    <div class="about-row">
      <span class="about-label">State</span>
      <span class="about-value">{statusInfo?.state ?? '--'}</span>
    </div>
    <div class="about-row">
      <span class="about-label">API Key</span>
      <span class="about-value">{statusInfo?.has_key ? 'Configured' : 'Not set'}</span>
    </div>
  </div>

  <div class="about-card">
    <h3>Features</h3>
    <ul class="feature-list">
      <li>Hold-to-record and toggle modes</li>
      <li>Double-tap for hands-free dictation</li>
      <li>OpenAI Whisper or local STT backend</li>
      <li>LLM-powered text cleanup and formatting</li>
      <li>Custom dictionary for domain-specific terms</li>
      <li>Text expansion snippets</li>
      <li>App-context-aware formatting</li>
      <li>Clipboard, wtype, or ydotool output</li>
    </ul>
  </div>

  <div class="about-card">
    <h3>Credits</h3>
    <div class="about-row">
      <span class="about-label">Author</span>
      <span class="about-value">Simon Hoffmann</span>
    </div>
    <div class="about-row">
      <span class="about-label">Built with</span>
      <span class="about-value">Go, Wails, Svelte</span>
    </div>
    <div class="about-row">
      <span class="about-label">License</span>
      <span class="about-value">Private</span>
    </div>
  </div>
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

  .about-hero {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 32px 0 24px;
    gap: 12px;
  }

  .about-icon {
    margin-bottom: 4px;
  }

  .about-title {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }

  .about-title h1 {
    font-size: 24px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 0.5px;
  }

  .about-version {
    font-size: 13px;
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  .about-description {
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
    max-width: 400px;
  }

  .about-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-bottom: 12px;
  }

  .about-card h3 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
  }

  .about-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .about-label {
    color: var(--text-secondary);
    font-size: 12px;
  }

  .about-value {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .feature-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .feature-list li {
    font-size: 12px;
    color: var(--text-secondary);
    padding-left: 16px;
    position: relative;
  }

  .feature-list li::before {
    content: '';
    position: absolute;
    left: 0;
    top: 6px;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    opacity: 0.6;
  }
</style>
