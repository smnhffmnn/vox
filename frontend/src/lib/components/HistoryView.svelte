<script lang="ts">
  import { onMount } from 'svelte'
  import { GetHistory } from '../api'
  import type { HistoryEntry } from '../api'

  let entries = $state<HistoryEntry[]>([])
  let expandedIndex = $state<number | null>(null)
  let loading = $state(true)

  onMount(async () => {
    try {
      const data = await GetHistory()
      entries = data
    } catch (e) {
      console.error('Failed to load history:', e)
    } finally {
      loading = false
    }
  })

  async function refresh() {
    loading = true
    try {
      const data = await GetHistory()
      entries = data
    } catch (e) {
      console.error('Failed to refresh history:', e)
    } finally {
      loading = false
    }
  }

  function toggleExpand(index: number) {
    expandedIndex = expandedIndex === index ? null : index
  }

  function formatTimestamp(ts: string): string {
    try {
      const d = new Date(ts)
      return d.toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    } catch {
      return ts
    }
  }

  function formatDuration(sec: number): string {
    if (sec < 1) return '<1s'
    return `${sec.toFixed(1)}s`
  }

  function hasRawDiff(entry: HistoryEntry): boolean {
    return entry.raw_text !== entry.cleaned_text && entry.raw_text.length > 0
  }
</script>

<div class="section">
  <div class="section-header">
    <h2>History</h2>
    <button onclick={refresh} disabled={loading}>
      {loading ? 'Loading...' : 'Refresh'}
    </button>
  </div>

  {#if loading && entries.length === 0}
    <div class="loading">Loading history...</div>
  {:else if entries.length === 0}
    <div class="empty">No transcriptions yet. Start dictating to see history here.</div>
  {:else}
    <div class="history-list">
      {#each entries as entry, i}
        <div class="history-item" class:expanded={expandedIndex === i}>
          <button class="history-header" onclick={() => toggleExpand(i)}>
            <div class="history-meta">
              <span class="timestamp">{formatTimestamp(entry.timestamp)}</span>
              <span class="duration">{formatDuration(entry.duration_seconds)}</span>
              {#if entry.app_context}
                <span class="app-context">{entry.app_context}</span>
              {/if}
              <span class="backend badge">{entry.backend}</span>
            </div>
            <p class="history-text">{entry.cleaned_text}</p>
            {#if hasRawDiff(entry)}
              <svg
                class="expand-icon"
                class:rotated={expandedIndex === i}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                width="14"
                height="14"
              >
                <path d="M6 9l6 6 6-6" />
              </svg>
            {/if}
          </button>

          {#if expandedIndex === i && hasRawDiff(entry)}
            <div class="history-detail">
              <div class="detail-label">Raw transcription</div>
              <p class="detail-text">{entry.raw_text}</p>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .section {
    margin-top: 4px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
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

  .empty {
    color: var(--text-muted);
    padding: 32px;
    text-align: center;
    border: 1px dashed var(--border);
    border-radius: var(--radius);
    font-size: 12px;
  }

  .history-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .history-item {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .history-header {
    width: 100%;
    text-align: left;
    padding: 12px 16px;
    border: none;
    background: none;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 6px;
    position: relative;
    font-size: 13px;
    color: var(--text);
  }

  .history-header:hover {
    background: var(--bg-surface-hover);
  }

  .history-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .timestamp {
    font-size: 11px;
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  .duration {
    font-size: 11px;
    color: var(--text-secondary);
    font-family: var(--font-mono);
  }

  .app-context {
    font-size: 10px;
    padding: 1px 6px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
  }

  .history-text {
    font-size: 13px;
    line-height: 1.4;
    color: var(--text);
  }

  .expand-icon {
    position: absolute;
    top: 14px;
    right: 14px;
    color: var(--text-muted);
    transition: transform 0.15s ease;
  }

  .expand-icon.rotated {
    transform: rotate(180deg);
  }

  .history-detail {
    padding: 0 16px 12px;
    border-top: 1px solid var(--border);
    margin-top: 0;
    padding-top: 10px;
  }

  .detail-label {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-muted);
    margin-bottom: 4px;
  }

  .detail-text {
    font-size: 12px;
    line-height: 1.4;
    color: var(--text-secondary);
    font-family: var(--font-mono);
  }
</style>
