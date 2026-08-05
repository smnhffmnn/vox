<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { GetLogs, ClearLogs, CopyToClipboard, EventsOn } from '../api'
  import type { LogRecord } from '../api'

  let records = $state<LogRecord[]>([])
  let level = $state<'all' | 'error' | 'warn'>('all')
  let loading = $state(true)
  let copied = $state(false)
  // This view exists to make failures visible, so its own failures cannot go to
  // the console — nobody opens DevTools in a desktop app.
  let notice = $state<string | null>(null)
  let cancelLogEvent: (() => void) | undefined
  let copiedTimer: ReturnType<typeof setTimeout> | undefined

  const filtered = $derived.by(() => {
    if (level === 'all') return records
    if (level === 'error') return records.filter((r) => r.level === 'error')
    return records.filter((r) => r.level === 'error' || r.level === 'warn')
  })

  onMount(() => {
    load()
    // Prepend the incoming record instead of refetching everything: a burst of
    // errors would otherwise trigger a refetch storm on the very view that is
    // supposed to show them.
    cancelLogEvent = EventsOn<LogRecord>('log-error', (rec) => {
      // Cap on insert, mirroring the backend's 500-record ring buffer: without
      // it a long-running app with recurring errors grows this array unbounded
      // until the next manual refresh.
      if (rec?.message) records = [rec, ...records].slice(0, 500)
    })
  })

  onDestroy(() => {
    if (cancelLogEvent) cancelLogEvent()
    if (copiedTimer) clearTimeout(copiedTimer)
  })

  async function load() {
    loading = true
    try {
      records = await GetLogs()
      notice = null
    } catch (e) {
      notice = `Could not load the log: ${e}`
    } finally {
      loading = false
    }
  }

  async function clear() {
    try {
      await ClearLogs()
      records = []
      notice = null
    } catch (e) {
      notice = `Could not clear the log: ${e}`
    }
  }

  async function copyAll() {
    const text = filtered.map((r) => `${r.time} [${r.level}/${r.step}] ${r.message}`).join('\n')
    try {
      await CopyToClipboard(text)
      copied = true
      if (copiedTimer) clearTimeout(copiedTimer)
      copiedTimer = setTimeout(() => (copied = false), 2000)
      notice = null
    } catch (e) {
      notice = `Could not copy the log: ${e}`
    }
  }

  function formatTime(ts: string): string {
    try {
      return new Date(ts).toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    } catch {
      return ts
    }
  }
</script>

<div class="section">
  <div class="section-header">
    <h2>Diagnostics</h2>
    <div class="header-actions">
      <button onclick={copyAll} disabled={filtered.length === 0}>
        {copied ? 'Copied' : 'Copy'}
      </button>
      <button onclick={clear} disabled={records.length === 0}>Clear</button>
      <button onclick={load} disabled={loading}>{loading ? 'Loading...' : 'Refresh'}</button>
    </div>
  </div>

  <p class="hint">
    What the app did and why something failed. Kept in memory only — the newest 500 entries since
    the app started.
  </p>

  {#if notice}
    <div class="notice">{notice}</div>
  {/if}

  <div class="filters">
    {#each [['all', 'All'], ['warn', 'Warnings & errors'], ['error', 'Errors']] as [value, label]}
      <button class="filter" class:active={level === value} onclick={() => (level = value as typeof level)}>
        {label}
      </button>
    {/each}
  </div>

  {#if loading && records.length === 0}
    <div class="loading">Loading...</div>
  {:else if filtered.length === 0}
    <div class="empty">
      {records.length === 0 ? 'Nothing logged yet.' : 'No entry matches this filter.'}
    </div>
  {:else}
    <div class="log">
      <!-- No key: records carry no stable id, and a key built from the index
           would change for every row whenever a new record arrives. -->
      {#each filtered as r}
        <div class="row" class:error={r.level === 'error'} class:warn={r.level === 'warn'}>
          <span class="time">{formatTime(r.time)}</span>
          <span class="step">{r.step}</span>
          <span class="message">{r.message}</span>
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
    margin-bottom: 8px;
  }

  .section-header h2 {
    font-size: 15px;
    font-weight: 600;
  }

  .header-actions {
    display: flex;
    gap: 6px;
  }

  .hint {
    font-size: 11px;
    line-height: 1.5;
    color: var(--text-muted);
    margin-bottom: 12px;
  }

  .notice {
    padding: 7px 10px;
    margin-bottom: 10px;
    border: 1px solid var(--red);
    border-radius: var(--radius);
    color: var(--red);
    font-size: 12px;
  }

  .filters {
    display: flex;
    gap: 4px;
    margin-bottom: 12px;
  }

  .filter {
    padding: 3px 9px;
    font-size: 11px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 5px;
    color: var(--text-secondary);
  }

  .filter.active {
    color: var(--text);
    border-color: var(--accent);
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

  .log {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .row {
    display: grid;
    grid-template-columns: 72px 82px 1fr;
    gap: 8px;
    padding: 6px 10px;
    font-size: 11px;
    font-family: var(--font-mono);
    line-height: 1.5;
    border-bottom: 1px solid var(--border);
    color: var(--text-secondary);
  }

  .row:last-child {
    border-bottom: none;
  }

  .row.warn {
    color: var(--yellow);
    background: var(--yellow-bg);
  }

  .row.error {
    color: var(--red);
    background: var(--red-bg);
  }

  .time {
    color: var(--text-muted);
  }

  .step {
    text-transform: uppercase;
    font-size: 10px;
    letter-spacing: 0.3px;
    opacity: 0.8;
  }

  .message {
    word-break: break-word;
    white-space: pre-wrap;
  }
</style>
