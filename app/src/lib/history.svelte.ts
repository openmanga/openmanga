// Undo/redo of layer edits through the engine's per-layer history (images/.history).
import { sb, refresh } from './sb.svelte'

type Entry = { target: string[]; layer: string }
const undoStack: Entry[] = []
const redoStack: Entry[] = []
export const hist = $state({ canUndo: false, canRedo: false })

const sync = () => {
  hist.canUndo = undoStack.length > 0
  hist.canRedo = redoStack.length > 0
}

/** target: [boardUid] or ['--page', n] */
export function pushEdit(target: string[], layer: string) {
  undoStack.push({ target, layer })
  if (undoStack.length > 20) undoStack.shift() // the engine keeps 20 versions per layer
  redoStack.length = 0
  sync()
}

export function clearHistory() {
  undoStack.length = 0
  redoStack.length = 0
  sync()
}

export async function undo() {
  const e = undoStack.pop()
  if (!e) return
  sync()
  await sb(['layer', 'undo', ...e.target, e.layer]).catch(() => {})
  redoStack.push(e)
  sync()
  await refresh()
}

export async function redo() {
  const e = redoStack.pop()
  if (!e) return
  sync()
  await sb(['layer', 'redo', ...e.target, e.layer]).catch(() => {})
  undoStack.push(e)
  sync()
  await refresh()
}
