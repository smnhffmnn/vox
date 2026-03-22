<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { activeView, status, appState } from './lib/stores'
  import { EventsOn } from './lib/api'
  import type { View } from './lib/stores'
  import StatusView from './lib/components/StatusView.svelte'
  import SettingsView from './lib/components/SettingsView.svelte'
  import BackendsView from './lib/components/BackendsView.svelte'
  import DictionaryView from './lib/components/DictionaryView.svelte'
  import SnippetsView from './lib/components/SnippetsView.svelte'
  import HistoryView from './lib/components/HistoryView.svelte'
  import AboutView from './lib/components/AboutView.svelte'

  let cleanupStateEvent: (() => void) | undefined

  const navItems: { id: View; label: string }[] = [
    { id: 'status', label: 'Status' },
    { id: 'settings', label: 'Settings' },
    { id: 'backends', label: 'Backends' },
    { id: 'dictionary', label: 'Dictionary' },
    { id: 'snippets', label: 'Snippets' },
    { id: 'history', label: 'History' },
    { id: 'about', label: 'About' },
  ]

  function navIcon(id: View): string {
    switch (id) {
      case 'status': return 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 14a4 4 0 110-8 4 4 0 010 8z'
      case 'settings': return 'M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 00.12-.61l-1.92-3.32a.49.49 0 00-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.484.484 0 00-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.07.62-.07.94s.02.64.07.94l-2.03 1.58a.49.49 0 00-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6A3.6 3.6 0 1112 8.4a3.6 3.6 0 010 7.2z'
      case 'backends': return 'M4 1h16a2 2 0 012 2v4a2 2 0 01-2 2H4a2 2 0 01-2-2V3a2 2 0 012-2zm0 10h16a2 2 0 012 2v4a2 2 0 01-2 2H4a2 2 0 01-2-2v-4a2 2 0 012-2zm2-7a1 1 0 100 2 1 1 0 000-2zm0 10a1 1 0 100 2 1 1 0 000-2z'
      case 'dictionary': return 'M4 19.5A2.5 2.5 0 016.5 17H20V2H6.5A2.5 2.5 0 004 4.5v15zm2-12a.5.5 0 01.5-.5H18v10H6.5c-.53 0-1.04.11-1.5.3V7.5zM6.5 19a.5.5 0 010-1H18v1H6.5z'
      case 'snippets': return 'M13 10V3L4 14h7v7l9-11h-7z'
      case 'history': return 'M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10 10-4.5 10-10S17.5 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67V7z'
      case 'about': return 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z'
    }
  }

  function stateColor(state: string): string {
    switch (state) {
      case 'recording': return 'var(--red)'
      case 'processing': return 'var(--orange)'
      default: return 'var(--text-muted)'
    }
  }

  function statePulsing(state: string): boolean {
    return state === 'recording' || state === 'processing'
  }

  onMount(() => {
    cleanupStateEvent = EventsOn('state-changed', (data: any) => {
      $appState = typeof data === 'string' ? data : data.state
    })
  })

  onDestroy(() => {
    if (cleanupStateEvent) cleanupStateEvent()
  })
</script>

<div class="app">
  <div class="sidebar">
    <div class="sidebar-header drag">
      <div class="sidebar-title">
        <span class="logo">Vox</span>
        <span
          class="state-dot"
          class:pulse={statePulsing($appState)}
          style:background={stateColor($appState)}
        ></span>
      </div>
      {#if $status?.version}
        <span class="version">v{$status.version}</span>
      {/if}
    </div>
    <nav class="sidebar-nav">
      {#each navItems as item}
        <button
          class="nav-item"
          class:active={$activeView === item.id}
          onclick={() => ($activeView = item.id)}
        >
          <svg class="nav-icon" viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
            <path d={navIcon(item.id)} />
          </svg>
          <span class="nav-label">{item.label}</span>
        </button>
      {/each}
    </nav>
  </div>

  <main class="content">
    {#if $activeView === 'status'}
      <StatusView />
    {:else if $activeView === 'settings'}
      <SettingsView />
    {:else if $activeView === 'backends'}
      <BackendsView />
    {:else if $activeView === 'dictionary'}
      <DictionaryView />
    {:else if $activeView === 'snippets'}
      <SnippetsView />
    {:else if $activeView === 'history'}
      <HistoryView />
    {:else if $activeView === 'about'}
      <AboutView />
    {/if}
  </main>
</div>

<style>
  .app {
    display: flex;
    height: 100%;
  }

  .sidebar {
    width: 220px;
    flex-shrink: 0;
    background: var(--bg-surface);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
  }

  .sidebar-header {
    padding: 44px 16px 12px;
    border-bottom: 1px solid var(--border);
  }

  .sidebar-title {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .logo {
    font-size: 18px;
    font-weight: 700;
    letter-spacing: 0.5px;
    color: var(--accent);
  }

  .state-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    transition: background 0.3s ease;
  }

  .state-dot.pulse {
    animation: -global-pulse 1.5s ease-in-out infinite;
  }

  .version {
    display: block;
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 2px;
  }

  .sidebar-nav {
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border: none;
    background: none;
    border-radius: 6px;
    color: var(--text-secondary);
    font-size: 13px;
    text-align: left;
    width: 100%;
  }

  .nav-item:hover {
    background: var(--bg-surface-hover);
    color: var(--text);
  }

  .nav-item.active {
    background: var(--bg-inset);
    color: var(--text);
    font-weight: 500;
  }

  .nav-icon {
    width: 16px;
    height: 16px;
    flex-shrink: 0;
    opacity: 0.7;
  }

  .nav-item.active .nav-icon {
    opacity: 1;
    color: var(--accent);
  }

  .nav-label {
    flex: 1;
  }

  .content {
    flex: 1;
    overflow-y: auto;
    padding: 24px 28px;
  }
</style>
