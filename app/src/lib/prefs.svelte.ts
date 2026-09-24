// pref.json through `sb prefs`; changes are broadcast so the editor window follows the Settings window.
import { emit, listen } from '@tauri-apps/api/event'
import { sb } from './sb.svelte'

export const prefs = $state<Record<string, any>>({})

// app-only keys (sb keeps unknown keys) and their defaults
export const DEFAULTS: Record<string, any> = {
  appearance: 'dark',
  enableAutoSave: true,
  enableTooltips: true,
  enableUISoundEffects: false,
  straightLineDelayInMsecs: 650,
  enableHighQualityDrawingEngine: true,
  tiltChangesWidth: true,
  pressureCurve: 'soft',
  toolSizes: {},
  mangaDefaultPageSize: 'b5',
  mangaReadingDirection: 'rtl',
  mangaStartingPages: 1,
  guides: { grid: false, center: false, thirds: false, safe: false },
  gridSize: 50,
  guideColor: '#0A84FF',
  onionOpacity: 0.35,
  deskColor: 'match',
  pageShadow: true,
}

export const pref = <T = any>(k: string): T => (prefs[k] ?? DEFAULTS[k]) as T

export async function loadPrefs() {
  const p = await sb<any>(['prefs', 'get'], undefined, false)
  Object.assign(prefs, p)
}

export async function setPref(key: string, value: any) {
  prefs[key] = value
  await sb(['prefs', 'set', key, JSON.stringify(value)], undefined, false)
  await emit('prefs-changed', { key, value })
}

export function followPrefs(onChange?: (key: string) => void) {
  return listen<{ key: string; value: any }>('prefs-changed', (e) => {
    prefs[e.payload.key] = e.payload.value
    onChange?.(e.payload.key)
  })
}

/** Pressure curve from Settings › Drawing: soft lifts light pressure, firm needs more. */
export function curve(p: number) {
  const g = { soft: 0.6, linear: 1, firm: 1.7 }[pref<string>('pressureCurve')] ?? 1
  return Math.pow(p, g)
}
