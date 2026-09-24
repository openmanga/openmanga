// Bridge to the `sb` sidecar and the app state built from its JSON.
import { invoke, convertFileSrc } from '@tauri-apps/api/core'
import { listen } from '@tauri-apps/api/event'

export type Pt = [number, number]
export type Placement = { board: string; x: number; y: number; scale: number; rotation: number; fit: 'fill' | 'fit' | 'none' }
export type Panel = { id: string; order: number; points: Pt[]; box: [number, number, number, number]; border: number; bleed: boolean; content: Placement | null }
export type BalloonType = 'speech' | 'thought' | 'shout' | 'whisper' | 'narration' | 'sfx'
export type Balloon = { id: string; type: BalloonType; text: string; fontSize: number; vertical: boolean; x: number; y: number; w: number; h: number; tail?: Pt; panel?: string }
export type PageInfo = { id: string; page: number; panelOrder: 'auto' | 'manual'; panels: Panel[]; balloons: Balloon[]; layers: string[]; size: Pt; spread: number[]; posterframe: string; thumbnail: string }
export type PageSummary = { id: string; page: number; panels: number; balloons: number; spread: number[]; posterframe: string; thumbnail: string }
export type Board = {
  uid: string; number: number; name: string; description: string; notes: string
  size: Pt; base: string; posterframe: string; thumbnail: string
}

export const LAYERS = ['reference', 'fill', 'tone', 'pencil', 'ink', 'notes'] as const
export type Layer = (typeof LAYERS)[number]

export class SbError extends Error {
  constructor(public code: string, message: string) { super(message) }
}

/** Runs `sb <args> --json` (with `--project` for the open project). */
export async function sb<T = any>(args: (string | number)[], stdin?: string, withProject = true): Promise<T> {
  const a = args.map(String)
  if (withProject && app.project) a.push('--project', app.project.file)
  try {
    return await invoke<T>('sb', { args: a, stdin: stdin ?? null, cwd: null })
  } catch (e: any) {
    throw new SbError(e?.code ?? 'error', e?.message ?? String(e))
  }
}

/** Runs an editing command, then reloads the project JSON. */
export async function edit<T = any>(args: (string | number)[], stdin?: string): Promise<T> {
  try {
    const r = await sb<T>(args, stdin)
    await refresh()
    return r
  } catch (e: any) {
    app.error = e.message
    throw e
  }
}

// ---- file urls, cache-busted per path when the watcher reports a change ----
const revs = $state<Record<string, number>>({})
export function fileUrl(path: string | undefined): string {
  if (!path) return ''
  return convertFileSrc(path) + '?v=' + (revs[path] ?? 0)
}
export function bump(paths: string[]) {
  const t = Date.now()
  for (const p of paths) revs[p] = t
}

// ---- app state ----
export type View = 'page' | 'spread' | 'grid' | 'board'
export type Mode = 'draw' | 'panel' | 'balloon'
export type Sheet = null | 'new' | 'export' | 'import'

export const app = $state({
  project: null as null | { file: string; dir: string; title: string; aspect: number },
  pages: [] as PageSummary[],
  pageSize: [1414, 2000] as Pt,
  reading: 'rtl' as 'rtl' | 'ltr',
  boards: [] as Board[],
  page: null as PageInfo | null,
  pageNo: 1,
  boardNo: 1,
  view: 'page' as View,
  mode: 'draw' as Mode,
  sidebarTab: 'pages' as 'pages' | 'boards',
  sidebar: true,
  inspector: true,
  sheet: null as Sheet,
  selPanel: null as string | null,
  selPanels: [] as string[],
  selBalloon: null as string | null,
  focusPanel: null as string | null,
  error: '' as string,
  /** layers hidden in the view (board and page share the switches) */
  hidden: { tone: false } as Record<string, boolean>,
  onion: false,
  /** board being dragged from the Boards library onto a panel */
  drag: null as null | { uid: string; x: number; y: number; panel: string | null },
  zoom: 1,
  /** opacity/existence of the current board's layers (`sb layer list`) */
  layerInfo: {} as Record<string, { opacity: number; exists: boolean }>,
  fitRequest: 0,
  /** gutter px for panel splits (0 = engine default) */
  gutter: 0,
  popover: null as null | 'templates',
  /** bumped to focus the balloon text field in the inspector */
  editText: 0,
})

export function boardByUid(uid: string | undefined) {
  return app.boards.find((b) => b.uid === uid)
}
export const curBoard = () => app.boards[app.boardNo - 1]
export const firstPageSingle = () => app.pages[0]?.spread.length === 1
export const boardTitle = (b: Board | undefined) => (b ? b.name || `Board ${b.number}` : '')

let stopWatch: (() => void) | null = null

export async function openProject(file: string, start: View = 'page') {
  const info = await sb<any>(['project', 'open', file], undefined, false)
  const scene = info.scene ?? info
  const f: string = scene.file ?? file
  const dir = f.slice(0, f.lastIndexOf('/'))
  if (scene.mode !== 'manga') await sb(['project', 'set-mode', 'manga', '--project', f], undefined, false)
  app.project = { file: f, dir, title: f.slice(dir.length + 1).replace(/\.storyboarder$/, ''), aspect: scene.aspectRatio ?? 16 / 9 }
  Object.assign(app, { pageNo: 1, boardNo: 1, view: start, mode: 'draw', selPanel: null, selBalloon: null, focusPanel: null })
  app.sidebarTab = start === 'board' ? 'boards' : 'pages'
  await refresh()
  await invoke('watch', { dir })
  stopWatch?.()
  let timer: any
  stopWatch = await listen<string[]>('project-changed', (e) => {
    bump(e.payload)
    clearTimeout(timer)
    timer = setTimeout(refresh, 120)
  })
}

export async function closeProject() {
  stopWatch?.()
  stopWatch = null
  await invoke('watch', { dir: null })
  app.project = null
}

let refreshing: Promise<void> | null = null
let again = false
/** Reloads pages, boards and the current page from `sb` (coalesces overlapping calls). */
export async function refresh(): Promise<void> {
  if (!app.project) return
  if (refreshing) { again = true; return refreshing }
  refreshing = (async () => {
    do {
      again = false
      const [pl, bl] = await Promise.all([sb<any>(['page', 'list']), sb<any>(['board', 'list'])])
      app.pages = pl.pages
      app.pageSize = pl.pageSize
      app.reading = pl.readingDirection
      app.boards = loadBoards(bl.boards)
      app.pageNo = Math.min(Math.max(1, app.pageNo), Math.max(1, app.pages.length))
      app.boardNo = Math.min(Math.max(1, app.boardNo), Math.max(1, app.boards.length))
      app.page = app.pages.length ? await pageInfo(app.pageNo) : null
      await loadLayerInfo()
      if (app.selPanel && !app.page?.panels.some((p) => p.id === app.selPanel)) app.selPanel = null
      if (app.selBalloon && !app.page?.balloons.some((b) => b.id === app.selBalloon)) app.selBalloon = null
    } while (again)
  })().finally(() => (refreshing = null))
  return refreshing
}

function loadBoards(list: any[]): Board[] {
  const p = app.project!
  const defSize: Pt = [Math.round(900 * p.aspect), 900]
  return list.map((b) => {
    const base = b.posterframe.replace(/-posterframe\.jpg$/, '')
    return {
      uid: b.uid, number: b.number, name: b.name ?? '', description: b.description ?? '', notes: b.notes ?? '',
      size: b.size ?? defSize, base, posterframe: b.posterframe, thumbnail: b.thumbnail,
    }
  })
}

/** Opacity/existence of the drawing layers of the board or page on screen (`sb layer list`). */
export async function loadLayerInfo() {
  if (!app.project) return
  const b = app.boards[app.boardNo - 1]
  const args = app.view === 'board' ? (b ? [b.uid] : null) : app.page ? ['--page', app.page.page] : null
  if (!args) return
  const r = await sb<any>(['layer', 'list', ...args]).catch(() => null)
  if (!r) return
  const info: Record<string, { opacity: number; exists: boolean }> = {}
  for (const l of r.layers) info[l.name] = { opacity: l.opacity, exists: l.exists }
  app.layerInfo = info
}

export async function setPage(n: number) {
  app.pageNo = n
  app.selPanel = null
  app.selBalloon = null
  app.focusPanel = null
  app.page = await pageInfo(n)
}

/** `sb page info`, with empty lists as [] (the engine may print null). */
export async function pageInfo(n: number): Promise<PageInfo> {
  const p = await sb<PageInfo>(['page', 'info', n])
  p.panels ??= []
  p.balloons ??= []
  p.layers ??= []
  return p
}

// ---- page presets (page px at 2000 px high, like the default 1414x2000) ----
export const PAGE_PRESETS = [
  { id: 'b5', name: 'Manga B5', sub: '182×257 mm', w: 182, h: 257 },
  { id: 'b6', name: 'Tankōbon B6', sub: '128×182 mm', w: 128, h: 182 },
  { id: 'us', name: 'US Comic', sub: '6.63×10.25 in', w: 6.63, h: 10.25 },
  { id: 'a4', name: 'A4', sub: '210×297 mm', w: 210, h: 297 },
]
export const presetPx = (p: { w: number; h: number }): Pt => [Math.round((2000 * p.w) / p.h), 2000]
export function presetName(size: Pt) {
  const p = PAGE_PRESETS.find((p) => Math.abs(presetPx(p)[0] - size[0]) <= 2 && size[1] === 2000)
  return p?.name ?? `${size[0]}×${size[1]}`
}

/** Loads the new version of changed files first, then swaps them in (no blank frame while drawing). */
export async function bumpLoaded(paths: (string | undefined)[]) {
  const t = Date.now()
  const ps = paths.filter(Boolean) as string[]
  await Promise.all(
    ps.map((p) => {
      const img = new Image()
      img.src = convertFileSrc(p) + '?v=' + t
      return img.decode().catch(() => {})
    }),
  )
  for (const p of ps) revs[p] = t
}
