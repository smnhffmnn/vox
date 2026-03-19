<script lang="ts">
  import { onMount } from 'svelte'
  import { GetConfig, SaveConfig, TestSTT, TestLLM, SetAPIKey, DeleteAPIKey, HasAPIKey } from '../api'
  import type { ConfigResponse, TestResult } from '../api'

  let cfg = $state<ConfigResponse | null>(null)
  let saving = $state(false)
  let saved = $state(false)
  let error = $state('')

  // STT test
  let sttTesting = $state(false)
  let sttResult = $state<TestResult | null>(null)

  // LLM test
  let llmTesting = $state(false)
  let llmResult = $state<TestResult | null>(null)

  // API key
  let hasKey = $state(false)
  let newKey = $state('')
  let keySaving = $state(false)
  let keyMsg = $state('')
  let keyError = $state('')

  onMount(async () => {
    try {
      cfg = await GetConfig()
      hasKey = await HasAPIKey('openai-api-key')
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

  async function testStt() {
    sttTesting = true
    sttResult = null
    try {
      // Save config first so test uses current values
      if (cfg) await SaveConfig(cfg)
      sttResult = await TestSTT()
    } catch (e: any) {
      sttResult = { ok: false, status: 0, error: e?.message || 'Test failed' }
    } finally {
      sttTesting = false
    }
  }

  async function testLlm() {
    llmTesting = true
    llmResult = null
    try {
      if (cfg) await SaveConfig(cfg)
      llmResult = await TestLLM()
    } catch (e: any) {
      llmResult = { ok: false, status: 0, error: e?.message || 'Test failed' }
    } finally {
      llmTesting = false
    }
  }

  async function saveKey() {
    if (!newKey.trim()) return
    keySaving = true
    keyMsg = ''
    keyError = ''
    try {
      await SetAPIKey('openai-api-key', newKey.trim())
      hasKey = true
      newKey = ''
      keyMsg = 'API key saved'
      setTimeout(() => (keyMsg = ''), 2000)
    } catch (e: any) {
      keyError = e?.message || 'Failed to save key'
    } finally {
      keySaving = false
    }
  }

  async function deleteKey() {
    keySaving = true
    keyMsg = ''
    keyError = ''
    try {
      await DeleteAPIKey('openai-api-key')
      hasKey = false
      keyMsg = 'API key deleted'
      setTimeout(() => (keyMsg = ''), 2000)
    } catch (e: any) {
      keyError = e?.message || 'Failed to delete key'
    } finally {
      keySaving = false
    }
  }

  const sttBackends = [
    { value: 'openai', label: 'OpenAI (Whisper)' },
    { value: 'local', label: 'Local Server' },
  ]

  const llmBackends = [
    { value: 'openai', label: 'OpenAI' },
    { value: 'ollama', label: 'Ollama' },
    { value: 'none', label: 'None (raw output)' },
  ]
</script>

<div class="section">
  <div class="section-header">
    <h2>Backends</h2>
  </div>

  {#if !cfg}
    <div class="loading">Loading...</div>
  {:else}
    <!-- API Key Section -->
    <div class="card">
      <h3>API Key</h3>
      <div class="key-status">
        <span class="dot" class:green={hasKey} class:red={!hasKey}></span>
        <span class="key-label">{hasKey ? 'Configured in OS Keychain' : 'Not configured'}</span>
      </div>
      <div class="key-form">
        <input
          type="password"
          placeholder="sk-..."
          bind:value={newKey}
          onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') saveKey() }}
        />
        <button class="primary" onclick={saveKey} disabled={keySaving || !newKey.trim()}>
          Save Key
        </button>
        {#if hasKey}
          <button class="danger" onclick={deleteKey} disabled={keySaving}>
            Delete Key
          </button>
        {/if}
      </div>
      {#if keyMsg}
        <span class="success-msg">{keyMsg}</span>
      {/if}
      {#if keyError}
        <span class="error-msg">{keyError}</span>
      {/if}
    </div>

    <!-- STT Backend Section -->
    <div class="card">
      <h3>Speech-to-Text</h3>
      <div class="form-group">
        <label for="stt-backend">Backend</label>
        <select id="stt-backend" bind:value={cfg.stt_backend}>
          {#each sttBackends as b}
            <option value={b.value}>{b.label}</option>
          {/each}
        </select>
      </div>

      {#if cfg.stt_backend === 'local'}
        <div class="form-group">
          <label for="stt-url">Server URL</label>
          <input id="stt-url" type="text" placeholder="http://localhost:8080" bind:value={cfg.stt_url} />
        </div>
      {/if}

      <div class="test-row">
        <button onclick={testStt} disabled={sttTesting}>
          {sttTesting ? 'Testing...' : 'Test Connection'}
        </button>
        {#if sttResult}
          <span class="test-result" class:ok={sttResult.ok} class:fail={!sttResult.ok}>
            {sttResult.ok ? 'Connected' : sttResult.error || `HTTP ${sttResult.status}`}
          </span>
        {/if}
      </div>
    </div>

    <!-- LLM Backend Section -->
    <div class="card">
      <h3>LLM (Text Cleanup)</h3>
      <div class="form-group">
        <label for="llm-backend">Backend</label>
        <select id="llm-backend" bind:value={cfg.llm_backend}>
          {#each llmBackends as b}
            <option value={b.value}>{b.label}</option>
          {/each}
        </select>
      </div>

      {#if cfg.llm_backend === 'ollama'}
        <div class="form-group">
          <label for="llm-url">Server URL</label>
          <input id="llm-url" type="text" placeholder="http://localhost:11434/v1" bind:value={cfg.llm_url} />
        </div>
      {/if}

      {#if cfg.llm_backend !== 'none'}
        <div class="form-group">
          <label for="llm-model">Model</label>
          <input id="llm-model" type="text" placeholder="gpt-4o-mini" bind:value={cfg.llm_model} />
        </div>

        <div class="test-row">
          <button onclick={testLlm} disabled={llmTesting}>
            {llmTesting ? 'Testing...' : 'Test Connection'}
          </button>
          {#if llmResult}
            <span class="test-result" class:ok={llmResult.ok} class:fail={!llmResult.ok}>
              {llmResult.ok
                ? llmResult.message || 'Connected'
                : llmResult.error || `HTTP ${llmResult.status}`}
            </span>
          {/if}
        </div>
      {/if}
    </div>

    <div class="form-actions">
      <button class="primary" onclick={save} disabled={saving}>
        {saving ? 'Saving...' : 'Save Backend Config'}
      </button>
      {#if saved}
        <span class="success-msg">Saved</span>
      {/if}
      {#if error}
        <span class="error-msg">{error}</span>
      {/if}
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

  .card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px 20px;
    margin-bottom: 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .card h3 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
  }

  .key-status {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .key-label {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .key-form {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .key-form input {
    flex: 1;
    max-width: 300px;
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
    max-width: 320px;
  }

  .test-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .test-result {
    font-size: 12px;
    font-weight: 500;
  }

  .test-result.ok {
    color: var(--green);
  }

  .test-result.fail {
    color: var(--red);
  }

  .form-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 4px;
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
