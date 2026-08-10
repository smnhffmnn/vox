<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import {
    GetHistory,
    GetHistoryInfo,
    GetEntryAudio,
    RetryEntry,
    RevealEntryAudio,
    RevealHistoryFile,
    CopyToClipboard,
  } from '../api'
  import type { HistoryEntry, HistoryInfo } from '../api'

  // How many rows to render at once. The history holds up to `history_size`
  // entries (default 1000); rendering all of them makes the view sluggish.
  const PAGE = 100

  let entries = $state<HistoryEntry[]>([])
  let info = $state<HistoryInfo | null>(null)
  let query = $state('')
  let visible = $state(PAGE)
  let expandedId = $state<string | null>(null)
  let loading = $state(true)
  // One busy marker per entry: play, download and re-transcribe can overlap
  // across rows, and a single slot would let one finishing action re-enable
  // another entry's buttons while its request is still running.
  let busy = $state<string[]>([])
  let playingId = $state<string | null>(null)
  // Recordings are expensive to move across the bridge, so a fetched one is
  // reused for pause/resume and for the download.
  let audioCache = new Map<string, { url: string; filename: string }>()
  let notice = $state<{ kind: 'ok' | 'error'; text: string } | null>(null)
  let noticeTimer: ReturnType<typeof setTimeout> | undefined
  let audioEl: HTMLAudioElement | undefined

  const filtered = $derived.by(() => {
    const q = query.trim().toLowerCase()
    if (!q) return entries
    return entries.filter((e) =>
      [e.cleaned_text, e.raw_text, e.app_context, e.error_message]
        .join(' ')
        .toLowerCase()
        .includes(q),
    )
  })

  const shown = $derived(filtered.slice(0, visible))

  onMount(() => {
    load()
  })

  onDestroy(() => {
    if (noticeTimer) clearTimeout(noticeTimer)
  })

  async function load() {
    loading = true
    try {
      const [data, meta] = await Promise.all([GetHistory(), GetHistoryInfo()])
      entries = data
      info = meta
      // Drop cached recordings that no longer exist, so a pruned entry cannot
      // be played from a stale copy.
      for (const id of [...audioCache.keys()]) {
        if (!data.some((e) => e.id === id && e.has_audio)) audioCache.delete(id)
      }
    } catch (e) {
      flash('error', `Could not load history: ${e}`)
    } finally {
      loading = false
    }
  }

  function isBusy(id: string): boolean {
    return busy.includes(id)
  }

  function setBusy(id: string, on: boolean) {
    busy = on ? [...busy, id] : busy.filter((b) => b !== id)
  }

  async function audioURL(entry: HistoryEntry): Promise<{ url: string; filename: string } | null> {
    const cached = audioCache.get(entry.id)
    if (cached) return cached

    const data = await GetEntryAudio(entry.id)
    if (data.error) {
      flash('error', data.error)
      return null
    }
    const item = { url: `data:${data.mime_type};base64,${data.base64}`, filename: data.filename }
    audioCache.set(entry.id, item)
    return item
  }

  function flash(kind: 'ok' | 'error', text: string) {
    notice = { kind, text }
    if (noticeTimer) clearTimeout(noticeTimer)
    noticeTimer = setTimeout(() => (notice = null), 4000)
  }

  function toggleExpand(id: string) {
    expandedId = expandedId === id ? null : id
  }

  async function copy(text: string, label: string) {
    if (!text) return
    try {
      await CopyToClipboard(text)
      flash('ok', `${label} copied`)
    } catch (e) {
      flash('error', `Copy failed: ${e}`)
    }
  }

  async function togglePlay(entry: HistoryEntry) {
    if (!audioEl) return

    if (playingId === entry.id) {
      audioEl.pause()
      playingId = null
      return
    }
    if (isBusy(entry.id)) return

    setBusy(entry.id, true)
    try {
      const item = await audioURL(entry)
      if (!item) return
      audioEl.src = item.url
      await audioEl.play()
      playingId = entry.id
    } catch (e) {
      flash('error', `Playback failed: ${e}`)
    } finally {
      setBusy(entry.id, false)
    }
  }

  async function download(entry: HistoryEntry) {
    if (isBusy(entry.id)) return

    setBusy(entry.id, true)
    try {
      const item = await audioURL(entry)
      if (!item) return
      const a = document.createElement('a')
      a.href = item.url
      // Use the backend-computed name so the download filename is defined in one
      // place rather than re-derived here.
      a.download = item.filename
      a.click()
    } catch (e) {
      flash('error', `Download failed: ${e}`)
    } finally {
      setBusy(entry.id, false)
    }
  }

  async function retry(entry: HistoryEntry) {
    if (isBusy(entry.id)) return

    setBusy(entry.id, true)
    try {
      const res = await RetryEntry(entry.id, false)
      if (res.ok) {
        flash('ok', res.persisted ? 'Transcribed again' : `Transcribed again, but: ${res.error}`)
      } else {
        flash('error', res.error ?? 'Retry failed')
      }
      // Reload rather than patching the one row: a retry changes what is on
      // disk, so has_audio on other rows and the usage figures move too.
      await load()
    } catch (e) {
      flash('error', `Retry failed: ${e}`)
    } finally {
      setBusy(entry.id, false)
    }
  }

  async function reveal(entry: HistoryEntry) {
    try {
      await RevealEntryAudio(entry.id)
    } catch (e) {
      flash('error', `${e}`)
    }
  }

  async function revealFile() {
    try {
      await RevealHistoryFile()
    } catch (e) {
      flash('error', `${e}`)
    }
  }

  function formatTimestamp(ts: string): string {
    try {
      return new Date(ts).toLocaleString(undefined, {
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
    // Rounding can reach a full 60 (toFixed rounds 59.97s to "60.0", Math.round
    // rounds 119.6s to 60); carry into the next unit instead of showing "60.0s"
    // or "1m 60s".
    if (sec < 60) return sec >= 59.95 ? '1m 0s' : `${sec.toFixed(1)}s`
    const m = Math.floor(sec / 60)
    const s = Math.round(sec % 60)
    if (s === 60) return `${m + 1}m 0s`
    return `${m}m ${s}s`
  }

  function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  }

  function hasRawDiff(entry: HistoryEntry): boolean {
    return entry.raw_text !== entry.cleaned_text && entry.raw_text.length > 0
  }

  function summary(entry: HistoryEntry): string {
    if (entry.cleaned_text) return entry.cleaned_text
    if (entry.raw_text) return entry.raw_text
    if (entry.status === 'pending') return 'Transcription in progress.'
    if (entry.has_audio) return 'No text — the recording is kept, so it can be transcribed again.'
    return 'No text — the attempt failed before transcription.'
  }
</script>

<div class="section">
  <div class="section-header">
    <h2>History</h2>
    <button onclick={load} disabled={loading}>
      {loading ? 'Loading...' : 'Refresh'}
    </button>
  </div>

  <input
    class="search"
    type="search"
    placeholder="Search transcriptions..."
    bind:value={query}
    oninput={() => (visible = PAGE)}
  />

  {#if notice}
    <div class="notice" class:error={notice.kind === 'error'}>{notice.text}</div>
  {/if}

  {#if loading && entries.length === 0}
    <div class="loading">Loading history...</div>
  {:else if entries.length === 0}
    <div class="empty">No transcriptions yet. Start dictating to see history here.</div>
  {:else if filtered.length === 0}
    <div class="empty">No entry matches "{query}".</div>
  {:else}
    <div class="history-list">
      {#each shown as entry (entry.id)}
        <div
          class="history-item"
          class:failed={entry.status === 'failed'}
          class:pending={entry.status === 'pending'}
        >
          <button class="history-header" onclick={() => toggleExpand(entry.id)}>
            <div class="history-meta">
              <span class="timestamp">{formatTimestamp(entry.timestamp)}</span>
              <span class="duration">{formatDuration(entry.duration_seconds)}</span>
              {#if entry.app_context}
                <span class="app-context">{entry.app_context}</span>
              {/if}
              {#if entry.backend}
                <span class="backend badge">{entry.backend}</span>
              {/if}
              {#if entry.status === 'failed'}
                <span class="badge fail">failed{entry.failed_step ? `: ${entry.failed_step}` : ''}</span>
              {:else if entry.status === 'pending'}
                <span class="badge pending" title="Transcription had not finished">
                  in progress
                </span>
              {/if}
              {#if entry.suspected_hallucination}
                <span
                  class="badge suspected"
                  title="The transcript matched a known Whisper-hallucination pattern. It was inserted and stored anyway — double-check it."
                >
                  check text
                </span>
              {/if}
              {#if entry.has_audio}
                <span class="badge audio" title="Recording is still stored">audio</span>
              {/if}
            </div>
            <p class="history-text" class:muted={!entry.cleaned_text && !entry.raw_text}>
              {summary(entry)}
            </p>
            {#if entry.error_message}
              <p class="error-line">{entry.error_message}</p>
            {/if}
          </button>

          <div class="actions">
            {#if entry.cleaned_text}
              <button
                class="action"
                onclick={() => copy(entry.cleaned_text, 'Polished text')}
                title="Copy polished text"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
                  <path
                    d="M16 1H4a2 2 0 00-2 2v14h2V3h12V1zm3 4H8a2 2 0 00-2 2v14a2 2 0 002 2h11a2 2 0 002-2V7a2 2 0 00-2-2zm0 16H8V7h11v14z"
                  />
                </svg>
                <span>Copy</span>
              </button>
            {/if}

            {#if hasRawDiff(entry)}
              <button
                class="action"
                onclick={() => copy(entry.raw_text, 'Raw text')}
                title="Copy raw transcription"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
                  <path d="M3 5h18v2H3V5zm0 6h12v2H3v-2zm0 6h18v2H3v-2z" />
                </svg>
                <span>Copy raw</span>
              </button>
            {/if}

            {#if entry.has_audio && entry.status !== 'pending'}
              <button
                class="action"
                onclick={() => togglePlay(entry)}
                disabled={isBusy(entry.id)}
                title="Play recording"
              >
                {#if playingId === entry.id}
                  <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
                    <path d="M6 5h4v14H6V5zm8 0h4v14h-4V5z" />
                  </svg>
                  <span>Pause</span>
                {:else}
                  <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
                    <path d="M8 5v14l11-7L8 5z" />
                  </svg>
                  <span>Play</span>
                {/if}
              </button>

              <button
                class="action"
                onclick={() => retry(entry)}
                disabled={isBusy(entry.id)}
                title="Transcribe this recording again"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
                  <path
                    d="M17.65 6.35A8 8 0 106 18.36l1.42-1.42A6 6 0 1116 7.99l-2.5 2.5H20V4l-2.35 2.35z"
                  />
                </svg>
                <span>{isBusy(entry.id) ? 'Working...' : 'Re-transcribe'}</span>
              </button>

              <button
                class="action"
                onclick={() => download(entry)}
                disabled={isBusy(entry.id)}
                title="Download the WAV file"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
                  <path d="M12 16l-6-6h4V4h4v6h4l-6 6zm-7 2h14v2H5v-2z" />
                </svg>
                <span>Download</span>
              </button>

              <button class="action" onclick={() => reveal(entry)} title="Show the file on disk">
                <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
                  <path d="M10 4H2v16h20V6H12l-2-2z" />
                </svg>
                <span>Reveal</span>
              </button>
            {/if}

            {#if hasRawDiff(entry)}
              <button class="action ghost" onclick={() => toggleExpand(entry.id)}>
                {expandedId === entry.id ? 'Hide raw' : 'Show raw'}
              </button>
            {/if}
          </div>

          {#if expandedId === entry.id && hasRawDiff(entry)}
            <div class="history-detail">
              <div class="detail-label">Raw transcription</div>
              <p class="detail-text">{entry.raw_text}</p>
            </div>
          {/if}
        </div>
      {/each}
    </div>

    {#if filtered.length > shown.length}
      <button class="more" onclick={() => (visible += PAGE)}>
        Show more — {shown.length} of {filtered.length}
      </button>
    {/if}
  {/if}

  {#if info}
    <div class="retention">
      <p>
        Text is kept for the newest {info.text_kept} entries, recordings only for the newest
        {info.audio_kept}{#if info.usage_error}
          — the recordings directory could not be read, so the stored amount is unknown{:else}
          — {info.audio_files} stored right now, {formatBytes(info.audio_bytes)} on disk{/if}. Older
        entries keep their raw and polished text; only playback and re-transcribing need the
        recording.
      </p>
      {#if info.usage_error}
        <p class="usage-error">{info.usage_error}</p>
      {/if}
      <div class="retention-actions">
        <button class="action" onclick={revealFile} title="Show history.jsonl in the file manager">
          <svg viewBox="0 0 24 24" fill="currentColor" width="13" height="13">
            <path d="M10 4H2v16h20V6H12l-2-2z" />
          </svg>
          <span>Reveal history.jsonl</span>
        </button>
        <button
          class="action ghost"
          onclick={() => copy(info?.path ?? '', 'Path')}
          title="Copy the file path"
        >
          <span>Copy path</span>
        </button>
      </div>
      <code class="path">{info.path}</code>
    </div>
  {/if}
</div>

<!-- One shared player: the history can hold a thousand entries. -->
<audio bind:this={audioEl} onended={() => (playingId = null)} hidden></audio>

<style>
  .section {
    margin-top: 4px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }

  .section-header h2 {
    font-size: 15px;
    font-weight: 600;
  }

  .search {
    width: 100%;
    padding: 7px 10px;
    margin-bottom: 12px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    color: var(--text);
    font-size: 12px;
  }

  .notice {
    padding: 7px 10px;
    margin-bottom: 10px;
    border-radius: var(--radius);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    font-size: 12px;
    color: var(--text-secondary);
  }

  .notice.error {
    border-color: var(--red);
    color: var(--red);
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

  .history-item.failed {
    border-color: color-mix(in srgb, var(--red) 45%, var(--border));
  }

  .history-item.pending {
    border-color: color-mix(in srgb, var(--yellow) 45%, var(--border));
  }

  .badge.pending {
    color: var(--yellow);
    border-color: color-mix(in srgb, var(--yellow) 45%, var(--border));
  }

  .badge.suspected {
    color: var(--yellow);
    border-color: color-mix(in srgb, var(--yellow) 45%, var(--border));
  }

  .history-header {
    width: 100%;
    text-align: left;
    padding: 12px 16px 8px;
    border: none;
    background: none;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 6px;
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

  .badge.fail {
    color: var(--red);
    border-color: color-mix(in srgb, var(--red) 45%, var(--border));
  }

  .badge.audio {
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
  }

  .history-text {
    font-size: 13px;
    line-height: 1.4;
    color: var(--text);
  }

  .history-text.muted {
    color: var(--text-muted);
    font-style: italic;
  }

  .error-line {
    font-size: 11px;
    line-height: 1.4;
    color: var(--red);
    font-family: var(--font-mono);
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    padding: 0 12px 10px;
  }

  .action {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 8px;
    font-size: 11px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 5px;
    color: var(--text-secondary);
  }

  .action:hover:not(:disabled) {
    color: var(--text);
    border-color: var(--text-muted);
  }

  .action:disabled {
    opacity: 0.5;
  }

  .action.ghost {
    background: none;
    border-color: transparent;
  }

  .history-detail {
    padding: 0 16px 12px;
    border-top: 1px solid var(--border);
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

  .more {
    width: 100%;
    margin-top: 8px;
    padding: 7px;
    font-size: 12px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    color: var(--text-secondary);
  }

  .retention {
    margin-top: 20px;
    padding-top: 14px;
    border-top: 1px solid var(--border);
  }

  .retention p {
    font-size: 11px;
    line-height: 1.5;
    color: var(--text-muted);
    margin-bottom: 8px;
  }

  .retention-actions {
    display: flex;
    gap: 6px;
    margin-bottom: 8px;
  }

  .usage-error {
    color: var(--red);
    font-family: var(--font-mono);
  }

  .path {
    display: block;
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--text-muted);
    word-break: break-all;
  }
</style>
