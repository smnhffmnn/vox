<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { Events } from '@wailsio/runtime'

  let state = $state<string>('idle')
  let startedAt = $state<number>(0)
  let elapsed = $state<string>('0:00')
  let visible = $state(false)
  let timerInterval: ReturnType<typeof setInterval> | undefined
  let cancel: (() => void) | undefined

  function updateElapsed() {
    if (!startedAt) return
    const diff = Math.floor((Date.now() - startedAt) / 1000)
    const min = Math.floor(diff / 60)
    const sec = diff % 60
    elapsed = `${min}:${sec.toString().padStart(2, '0')}`
  }

  onMount(() => {
    cancel = Events.On('state-changed', (ev: any) => {
      const data = ev.data
      state = typeof data === 'string' ? data : (data.state ?? 'idle')

      if (state === 'recording') {
        if (data.started_at) startedAt = data.started_at
        else startedAt = Date.now()
        elapsed = '0:00'
        if (timerInterval) clearInterval(timerInterval)
        timerInterval = setInterval(updateElapsed, 200)
        visible = true
      } else if (state === 'processing') {
        if (timerInterval) clearInterval(timerInterval)
        timerInterval = undefined
        visible = true
      } else {
        if (timerInterval) clearInterval(timerInterval)
        timerInterval = undefined
        visible = false
      }
    })
  })

  onDestroy(() => {
    if (cancel) cancel()
    if (timerInterval) clearInterval(timerInterval)
  })
</script>

<div class="overlay" class:visible>
  {#if state === 'recording'}
    <div class="pill recording">
      <span class="dot"></span>
      <span class="time">{elapsed}</span>
      <span class="divider"></span>
      <span class="hint">Esc to cancel</span>
    </div>
  {:else if state === 'processing'}
    <div class="pill processing">
      <span class="spinner"></span>
      <span class="label">Processing…</span>
    </div>
  {/if}
</div>

<style>
  :global(html), :global(body) {
    margin: 0;
    padding: 0;
    background: transparent !important;
    overflow: hidden;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
    -webkit-font-smoothing: antialiased;
  }

  .overlay {
    display: flex;
    justify-content: center;
    align-items: center;
    width: 100%;
    height: 100%;
    opacity: 0;
    transform: translateY(-6px) scale(0.96);
    transition: opacity 0.2s ease, transform 0.2s ease;
    pointer-events: none;
  }

  .overlay.visible {
    opacity: 1;
    transform: translateY(0) scale(1);
  }

  .pill {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    border-radius: 20px;
    font-size: 13px;
    font-weight: 500;
    backdrop-filter: blur(24px);
    -webkit-backdrop-filter: blur(24px);
    white-space: nowrap;
    user-select: none;
  }

  .pill.recording {
    background: rgba(220, 50, 50, 0.88);
    color: #fff;
    box-shadow: 0 2px 16px rgba(220, 50, 50, 0.4), 0 0 0 0.5px rgba(255, 255, 255, 0.15) inset;
  }

  .pill.processing {
    background: rgba(40, 40, 40, 0.82);
    color: rgba(255, 255, 255, 0.92);
    box-shadow: 0 2px 16px rgba(0, 0, 0, 0.3), 0 0 0 0.5px rgba(255, 255, 255, 0.1) inset;
  }

  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: #fff;
    flex-shrink: 0;
    animation: pulse 1.2s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.35; transform: scale(0.8); }
  }

  .time {
    font-variant-numeric: tabular-nums;
    min-width: 28px;
    font-weight: 600;
    letter-spacing: 0.3px;
  }

  .divider {
    width: 1px;
    height: 14px;
    background: rgba(255, 255, 255, 0.3);
  }

  .hint {
    opacity: 0.65;
    font-size: 11px;
    font-weight: 400;
  }

  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255, 255, 255, 0.25);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    flex-shrink: 0;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .label {
    font-size: 13px;
    font-weight: 500;
  }

  @media (prefers-reduced-motion: reduce) {
    .dot { animation: none; }
    .spinner { animation: none; }
    .overlay { transition: none; }
  }

  @media (prefers-color-scheme: light) {
    .pill.processing {
      background: rgba(255, 255, 255, 0.88);
      color: rgba(30, 30, 30, 0.9);
      box-shadow: 0 2px 16px rgba(0, 0, 0, 0.12), 0 0 0 0.5px rgba(0, 0, 0, 0.08) inset;
    }
    .spinner {
      border-color: rgba(0, 0, 0, 0.12);
      border-top-color: rgba(30, 30, 30, 0.8);
    }
  }
</style>
