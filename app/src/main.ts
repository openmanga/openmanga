import { mount } from 'svelte'
import './tokens.css'
import './app.css'
import App from './App.svelte'
import Settings from './views/Settings.svelte'

const Root = location.hash === '#settings' ? Settings : App
export default mount(Root, { target: document.getElementById('app')! })

// Dev hook: runs new contents of $SB_UI_SCRIPT (see src-tauri ui_script); inert when unset.
import { invoke } from '@tauri-apps/api/core'
import * as sbState from './lib/sb.svelte'
import * as toolState from './lib/tools.svelte'
import * as history from './lib/history.svelte'
import { openSettings } from './lib/windows'
import { WebviewWindow } from '@tauri-apps/api/webviewWindow'
;(window as any).__sb = { ...sbState, ...toolState, ...history, openSettings, WebviewWindow }
const log = (...a: any[]) => invoke('ui_log', { msg: a.map((x) => (x instanceof Error ? x.stack : typeof x === 'object' ? JSON.stringify(x) : String(x))).join(' ') })
;(window as any).__log = log
// ui_log prints only while SB_UI_SCRIPT is set
const origError = console.error
console.error = (...a: any[]) => { origError(...a); log('error', ...a) }
addEventListener('error', (e) => log('error', e.message, e.filename, e.lineno))
addEventListener('unhandledrejection', (e) => log('rejection', e.reason))
let last = ''
const poll = async () => {
  const s = await invoke<string | null>('ui_script').catch(() => '')
  if (s === null) return // SB_UI_SCRIPT unset: no dev hook
  // scripts starting with //settings run in the Settings window, all others in the main window
  if (s && s !== last && s.startsWith('//settings') === (location.hash === '#settings')) {
    last = s
    try { await new Function(`return (async () => { ${s} })()`)(); log('script ok') } catch (e) { console.error(e) }
  }
  setTimeout(poll, 700)
}
poll()
