<script lang="ts">
  import { onMount } from 'svelte'
  import { GetConfig, SaveConfig } from '../api'
  import type { ConfigResponse } from '../api'

  let cfg = $state<ConfigResponse | null>(null)
  let saving = $state(false)
  let saved = $state(false)
  let error = $state('')

  onMount(async () => {
    try {
      cfg = await GetConfig()
    } catch (e) {
      error = 'Failed to load config'
      console.error(e)
    }
  })

  async function save() {
    if (!cfg) return
    saving = true
    error = ''
    saved = false
    try {
      await SaveConfig(cfg)
      saved = true
      setTimeout(() => (saved = false), 2000)
    } catch (e: any) {
      error = e?.message || 'Failed to save'
    } finally {
      saving = false
    }
  }

  const languages = [
    { value: 'de', label: 'German' },
    { value: 'en', label: 'English' },
    { value: 'es', label: 'Spanish' },
    { value: 'fr', label: 'French' },
    { value: 'it', label: 'Italian' },
    { value: 'pt', label: 'Portuguese' },
    { value: 'ja', label: 'Japanese' },
    { value: 'ko', label: 'Korean' },
    { value: 'zh', label: 'Chinese' },
  ]

  const outputMethods = [
    { value: 'clipboard', label: 'Clipboard' },
    { value: 'stdout', label: 'Stdout' },
    { value: 'wtype', label: 'wtype (Wayland)' },
    { value: 'ydotool', label: 'ydotool (X11/Wayland)' },
  ]

  const hotkeys = [
    { value: 'right_option', label: 'Right Option' },
    { value: 'f13', label: 'F13' },
    { value: 'f14', label: 'F14' },
    { value: 'f15', label: 'F15' },
    { value: 'f16', label: 'F16' },
    { value: 'f17', label: 'F17' },
    { value: 'f18', label: 'F18' },
    { value: 'f19', label: 'F19' },
    { value: 'f20', label: 'F20' },
  ]

  const modes = [
    { value: 'hold', label: 'Hold to record' },
    { value: 'toggle', label: 'Toggle on/off' },
  ]
</script>

<div class="section">
  <div class="section-header">
    <h2>Settings</h2>
  </div>

  {#if !cfg}
    <div class="loading">Loading settings...</div>
  {:else}
    <div class="form">
      <div class="form-group">
        <label for="language">Language</label>
        <select id="language" bind:value={cfg.language}>
          {#each languages as lang}
            <option value={lang.value}>{lang.label}</option>
          {/each}
        </select>
      </div>

      <div class="form-group">
        <label for="output">Output Method</label>
        <select id="output" bind:value={cfg.output}>
          {#each outputMethods as method}
            <option value={method.value}>{method.label}</option>
          {/each}
        </select>
      </div>

      <div class="form-group">
        <label for="hotkey">Hotkey</label>
        <select id="hotkey" bind:value={cfg.hotkey}>
          {#each hotkeys as hk}
            <option value={hk.value}>{hk.label}</option>
          {/each}
        </select>
      </div>

      <div class="form-group">
        <label for="mode">Mode</label>
        <select id="mode" bind:value={cfg.mode}>
          {#each modes as m}
            <option value={m.value}>{m.label}</option>
          {/each}
        </select>
      </div>

      <div class="form-group">
        <label for="doubletap">Double-Tap Window (ms)</label>
        <input id="doubletap" type="number" min="100" max="1000" step="50" bind:value={cfg.doubletap_window} />
        <span class="hint">Time window for detecting double-tap to toggle hands-free mode</span>
      </div>

      <div class="form-group">
        <label for="handsfree">Hands-Free Timeout (seconds)</label>
        <input id="handsfree" type="number" min="0" max="600" step="5" bind:value={cfg.handsfree_timeout} />
        <span class="hint">Auto-stop hands-free after this many seconds (0 = no limit)</span>
      </div>

      <div class="form-row">
        <div class="form-toggle">
          <label for="toggle-raw">Raw Mode</label>
          <button
            id="toggle-raw"
            class="toggle"
            class:active={cfg.raw}
            onclick={() => (cfg!.raw = !cfg!.raw)}
            aria-label="Toggle raw mode"
          ></button>
        </div>
        <span class="hint">Skip LLM cleanup, output raw transcription</span>
      </div>

      <div class="form-row">
        <div class="form-toggle">
          <label for="toggle-notifications">Notifications</label>
          <button
            id="toggle-notifications"
            class="toggle"
            class:active={cfg.notifications}
            onclick={() => (cfg!.notifications = !cfg!.notifications)}
            aria-label="Toggle notifications"
          ></button>
        </div>
        <span class="hint">Show system notifications after transcription</span>
      </div>

      <div class="form-row">
        <div class="form-toggle">
          <label for="toggle-audio">Audio Feedback</label>
          <button
            id="toggle-audio"
            class="toggle"
            class:active={cfg.audio_feedback}
            onclick={() => (cfg!.audio_feedback = !cfg!.audio_feedback)}
            aria-label="Toggle audio feedback"
          ></button>
        </div>
        <span class="hint">Play sounds when recording starts/stops</span>
      </div>

      <div class="form-actions">
        <button class="primary" onclick={save} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </button>
        {#if saved}
          <span class="success-msg">Saved</span>
        {/if}
        {#if error}
          <span class="error-msg">{error}</span>
        {/if}
      </div>
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

  .loading {
    color: var(--text-muted);
    padding: 24px;
    text-align: center;
  }

  .form {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .form-group label {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .form-group select,
  .form-group input {
    width: 100%;
    max-width: 320px;
  }

  .form-row {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .form-toggle {
    display: flex;
    align-items: center;
    justify-content: space-between;
    max-width: 320px;
  }

  .form-toggle label {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .hint {
    font-size: 11px;
    color: var(--text-muted);
  }

  .form-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-top: 8px;
    border-top: 1px solid var(--border);
  }

  .success-msg {
    font-size: 12px;
    color: var(--green);
    font-weight: 500;
  }

  .error-msg {
    font-size: 12px;
    color: var(--red);
  }
</style>
