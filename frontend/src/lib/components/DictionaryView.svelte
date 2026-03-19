<script lang="ts">
  import { onMount } from 'svelte'
  import { GetDictionary, SaveDictionary } from '../api'

  let words = $state<string[]>([])
  let newWord = $state('')
  let saving = $state(false)
  let saved = $state(false)
  let error = $state('')

  onMount(async () => {
    try {
      words = await GetDictionary()
    } catch (e) {
      console.error('Failed to load dictionary:', e)
    }
  })

  function addWord() {
    const w = newWord.trim()
    if (!w || words.includes(w)) return
    words = [...words, w]
    newWord = ''
  }

  function removeWord(index: number) {
    words = words.filter((_, i) => i !== index)
  }

  async function save() {
    saving = true
    error = ''
    saved = false
    try {
      await SaveDictionary(words)
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
    <h2>Dictionary</h2>
  </div>

  <p class="description">
    Custom words and names that the speech recognition should know. These are passed
    as prompt context to improve transcription accuracy for domain-specific terms.
  </p>

  <div class="card">
    <div class="add-form">
      <input
        type="text"
        placeholder="Add a word or name..."
        bind:value={newWord}
        onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') addWord() }}
      />
      <button onclick={addWord} disabled={!newWord.trim()}>Add</button>
    </div>

    {#if words.length === 0}
      <div class="empty">No custom words added yet.</div>
    {:else}
      <div class="word-list">
        {#each words as word, i}
          <div class="word-item">
            <span class="word-text">{word}</span>
            <button class="remove-btn" onclick={() => removeWord(i)} title="Remove">
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
        {saving ? 'Saving...' : 'Save Dictionary'}
      </button>
      <span class="word-count">{words.length} {words.length === 1 ? 'word' : 'words'}</span>
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

  .add-form input {
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

  .word-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .word-item {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px 4px 10px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 16px;
    font-size: 12px;
  }

  .word-text {
    color: var(--text);
  }

  .remove-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 2px;
    color: var(--text-muted);
    border-radius: 50%;
    width: 18px;
    height: 18px;
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

  .word-count {
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
