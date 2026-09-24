import { WebviewWindow } from '@tauri-apps/api/webviewWindow'

/** Opens (or focuses) the Settings window. */
export async function openSettings() {
  const w = await WebviewWindow.getByLabel('settings')
  if (w) return w.setFocus()
  new WebviewWindow('settings', {
    url: 'index.html#settings', title: 'Settings', width: 640, height: 640, resizable: false,
    titleBarStyle: 'overlay', hiddenTitle: true, theme: 'dark',
  })
}
