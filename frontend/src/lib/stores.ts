import { writable } from 'svelte/store'
import type { StatusResponse, ConfigResponse } from './api'

export type View = 'status' | 'settings' | 'backends' | 'dictionary' | 'snippets' | 'history' | 'about'

export const activeView = writable<View>('status')
export const status = writable<StatusResponse | null>(null)
export const appState = writable<string>('idle')
export const config = writable<ConfigResponse | null>(null)
export const loading = writable(false)
