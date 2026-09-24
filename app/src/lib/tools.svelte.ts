// Drawing tools (defaults from the engine, overridden by prefs.toolbar) and the keymap.
import { sb, type Layer } from './sb.svelte'

export type ToolId = 'light-pencil' | 'brush' | 'tone' | 'pencil' | 'pen' | 'note-pen' | 'eraser' | 'lasso'
export type ToolSettings = { size: number; opacity: number; color: string; palette: string[]; pressure: boolean }

// internal/draw/strokes.go Tools + internal/story/userdata.go ToolDefaults
const DEFAULTS: Record<Exclude<ToolId, 'lasso'>, ToolSettings & { layer: Layer | '' }> = {
  'light-pencil': { layer: 'reference', color: '#90CBF9', size: 20, opacity: 0.25, palette: ['#CFCFCF', '#9FA8DA', '#90CBF9'], pressure: true },
  brush: { layer: 'fill', color: '#90CBF9', size: 26, opacity: 0.7, palette: ['#4DABF5', '#607D8B', '#9E9E9E'], pressure: true },
  tone: { layer: 'tone', color: '#162A3F', size: 50, opacity: 0.15, palette: ['#162A3F', '#162A3F', '#162A3F'], pressure: true },
  pencil: { layer: 'pencil', color: '#121212', size: 4, opacity: 0.45, palette: ['#373737', '#223131', '#121212'], pressure: true },
  pen: { layer: 'ink', color: '#000000', size: 2, opacity: 0.9, palette: ['#373737', '#223131', '#000000'], pressure: true },
  'note-pen': { layer: 'notes', color: '#F44336', size: 8, opacity: 0.9, palette: ['#4CAF50', '#FF9800', '#F44336'], pressure: true },
  eraser: { layer: '', color: '#FFFFFF', size: 26, opacity: 1, palette: ['#FFFFFF', '#FFFFFF', '#FFFFFF'], pressure: true },
}

export const TOOL_NAMES: Record<ToolId, string> = {
  'light-pencil': 'Light Pencil', brush: 'Brush', tone: 'Tone', pencil: 'Pencil', pen: 'Pen', 'note-pen': 'Note Pen', eraser: 'Eraser', lasso: 'Lasso',
}

const hex = (n: number) => '#' + n.toString(16).padStart(6, '0').toUpperCase()

export const tools = $state({
  current: 'pencil' as ToolId,
  /** layer chosen in the layer list; null = the tool's own layer */
  layer: null as Layer | null,
  settings: Object.fromEntries(Object.entries(DEFAULTS).map(([k, v]) => [k, { ...v, palette: [...v.palette] }])) as Record<string, ToolSettings & { layer: Layer | '' }>,
  keymap: {} as Record<string, string>,
})

export const cur = () => tools.settings[tools.current === 'lasso' ? 'pencil' : tools.current]
export function toolLayer(): Layer {
  if (tools.layer) return tools.layer
  const l = cur().layer
  return (l || 'pencil') as Layer
}
export function selectTool(t: ToolId) {
  tools.current = t
  tools.layer = null
}

/** Loads prefs.toolbar.tools overrides and the keymap. */
export async function loadToolPrefs() {
  const [km, tb] = await Promise.all([
    sb<any>(['keymap', 'list'], undefined, false),
    sb<any>(['prefs', 'get', 'toolbar'], undefined, false).catch(() => null),
  ])
  tools.keymap = km.keymap
  const saved = tb?.value?.tools ?? {}
  for (const [id, s] of Object.entries<any>(saved)) {
    const t = tools.settings[id]
    if (!t) continue
    if (typeof s.color === 'number') t.color = hex(s.color)
    if (Array.isArray(s.palette)) t.palette = s.palette.map((c: number) => hex(c))
    if (typeof s.strokeOpacity === 'number') t.opacity = s.strokeOpacity
  }
}

let saveTimer: any
/** Persists color, palette and opacity of a tool (`sb prefs set-tool`). */
export function saveTool(id: string) {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    const t = tools.settings[id]
    sb(['prefs', 'set-tool', id, '--color', t.color, '--palette', t.palette.join(','), '--opacity', t.opacity.toFixed(2)], undefined, false)
  }, 400)
}

/** True when a keydown matches an Electron-style accelerator like "CommandOrControl+Shift+z". */
export function matches(e: KeyboardEvent, accel: string | undefined): boolean {
  if (!accel) return false
  const parts = accel.split('+')
  const key = parts.pop()!.toLowerCase()
  const mods = new Set(parts.map((p) => p.toLowerCase()))
  const cmd = mods.has('commandorcontrol') || mods.has('command') || mods.has('cmd')
  if (cmd !== e.metaKey || mods.has('shift') !== e.shiftKey || mods.has('alt') !== e.altKey) return false
  const k = e.key.toLowerCase()
  const names: Record<string, string> = { left: 'arrowleft', right: 'arrowright', up: 'arrowup', down: 'arrowdown', space: ' ', backspace: 'backspace', delete: 'delete', escape: 'escape', enter: 'enter' }
  if (names[key]) return k === names[key]
  // shifted digits/letters: compare the physical key
  return k === key || e.code.toLowerCase() === 'key' + key || e.code === 'Digit' + key
}

export const keyFor = (cmd: string) => tools.keymap[cmd]
