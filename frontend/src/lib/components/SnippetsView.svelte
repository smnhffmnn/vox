<script lang="ts">
  import { onMount } from 'svelte'
  import { GetSnippets, SaveSnippets } from '../api'
  import type { Snippet } from '../api'

  let snippets = $state<Snippet[]>([])
  let newTrigger = $state('')
  let newText = $state('')
  let saving = $state(false)
  let saved = $state(false)
  let error = $state('')

  onMount(async () => {
    try {
      snippets = await GetSnippets()
    } catch (e) {
      console.error('Failed to load snippets:', e)
    }
  })

  function addSnippet() {
    const trigger = newTrigger.trim()
    const text = newText.trim()
    if (!trigger || !text) return
    if (snippets.some(s => s.trigger === trigger)) return
    snippets = [...snippets, { trigger, text }]
    newTrigger = ''
    newText = ''
  }

  function removeSnippet(index: number) {
    snippets = snippets.filter((_, i) => i !== index)
  }

  async function save() {
    saving = true
    error = ''
    saved = false
    try {
      await SaveSnippets(snippets)
      saved = true
      setTimeout(() => (saved = false), 2000)
    } catch (e: any) {
      error = e?.message || 'Failed to save'
    } finally {
      saving = false
    }
  }
</script>

<div class="section">
  <div class="section-header">
    <h2>Snippets</h2>
  </div>

  <p class="description">
    Text expansion snippets. When a transcription exactly matches a trigger phrase,
    it gets replaced with the corresponding text. Useful for frequently dictated
    boilerplate like email signatures or addresses.
  </p>

  <div class="card">
    <div class="add-form">
      <input
        type="text"
        placeholder="Trigger phrase"
        bind:value={newTrigger}
        class="trigger-input"
      />
      <input
        type="text"
        placeholder="Replacement text"
        bind:value={newText}
        class="text-input"
        onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') addSnippet() }}
      />
      <button onclick={addSnippet} disabled={!newTrigger.trim() || !newText.trim()}>Add</button>
    </div>

    {#if snippets.length === 0}
      <div class="empty">No snippets configured yet.</div>
    {:else}
      <div class="snippet-list">
        {#each snippets as snippet, i}
          <div class="snippet-item">
            <div class="snippet-content">
              <span class="snippet-trigger">{snippet.trigger}</span>
              <svg class="arrow-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                <path d="M5 12h14M12 5l7 7-7 7" />
              </svg>
              <span class="snippet-text">{snippet.text}</span>
            </div>
            <button class="remove-btn" onclick={() => removeSnippet(i)} title="Remove">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
          </div>
        {/each}
      </div>
    {/if}

    <div class="form-actions">
      <button class="primary" onclick={save} disabled={saving}>
        {saving ? 'Saving...' : 'Save Snippets'}
      </button>
      <span class="snippet-count">{snippets.length} {snippets.length === 1 ? 'snippet' : 'snippets'}</span>
      {#if saved}
        <span class="success-msg">Saved</span>
      {/if}
      {#if error}
        <span class="error-msg">{error}</span>
      {/if}
    </div>
  </div>
</div>

<style>
  .section {
    margin-top: 4px;
  }

  .section-header {
    margin-bottom: 8px;
  }

  .section-header h2 {
    font-size: 15px;
    font-weight: 600;
  }

  .description {
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.5;
    margin-bottom: 16px;
  }

  .card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .add-form {
    display: flex;
    gap: 8px;
  }

  .trigger-input {
    width: 160px;
    flex-shrink: 0;
  }

  .text-input {
    flex: 1;
  }

  .empty {
    color: var(--text-muted);
    padding: 16px;
    text-align: center;
    border: 1px dashed var(--border);
    border-radius: var(--radius);
    font-size: 12px;
  }

  .snippet-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .snippet-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .snippet-content {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .snippet-trigger {
    font-weight: 600;
    font-size: 12px;
    color: var(--accent);
    white-space: nowrap;
  }

  .arrow-icon {
    flex-shrink: 0;
    color: var(--text-muted);
  }

  .snippet-text {
    font-size: 12px;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .remove-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 4px;
    color: var(--text-muted);
    border-radius: 4px;
    flex-shrink: 0;
  }

  .remove-btn:hover {
    color: var(--red);
    background: var(--red-bg);
  }

  .form-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-top: 8px;
    border-top: 1px solid var(--border);
  }

  .snippet-count {
    font-size: 11px;
    color: var(--text-muted);
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
