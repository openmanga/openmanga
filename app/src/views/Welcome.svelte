<script lang="ts">
  import { History, Library, Folder, Monitor, Cloud, LayoutGrid, List, ImagePlus, FolderOpen, Search, FilePlus2 } from 'lucide-svelte'
  import { open } from '@tauri-apps/plugin-dialog'
  import { homeDir } from '@tauri-apps/api/path'
  import { app, sb, openProject, fileUrl } from '../lib/sb.svelte'
  import { openSettings } from '../lib/windows'
  import NewProject from './NewProject.svelte'

  type Recent = { title: string; filename: string; time: number; pages?: number; boards?: number; thumb?: string; missing?: boolean }
  let recents = $state<Recent[]>([])
  let filter = $state<'recent' | 'all' | string>('recent')
  let listMode = $state(false)
  let query = $state('')
  let selected = $state('')
  let home = $state('')
  let importFiles = $state<string[] | null>(null)

  homeDir().then((h) => (home = h.replace(/\/$/, '')))
  const LOCATIONS = $derived([
    { id: `${home}/Manga`, name: 'Manga', icon: Folder },
    { id: `${home}/Desktop`, name: 'Desktop', icon: Monitor },
    { id: `${home}/Library/Mobile Documents/com~apple~CloudDocs`, name: 'iCloud Drive', icon: Cloud },
  ])

  async function load() {
    const r = await sb<any>(['recent', 'list'], undefined, false)
    recents = r.recent.map((x: any) => ({
      title: x.title, filename: x.filename, time: x.time, pages: x.pages, boards: x.boards, missing: x.exists === false,
      // the 120 px thumbnail is too small for the cards; the posterframe sits next to it
      thumb: x.thumbnail?.replace(/-thumbnail\.png$/, '-posterframe.jpg'),
    }))
  }
  load()

  const shown = $derived(
    recents
      .filter((r) => filter === 'recent' || filter === 'all' || r.filename.startsWith(filter + '/'))
      .filter((r) => !query || r.title.toLowerCase().includes(query.toLowerCase())),
  )

  function when(t: number) {
    const d = new Date(t), now = new Date()
    const hours = (now.getTime() - t) / 36e5
    if (hours < 1) return 'just now'
    if (hours < 24 && d.getDate() === now.getDate()) return `${Math.floor(hours)} ${Math.floor(hours) === 1 ? 'hour' : 'hours'} ago`
    if (hours < 48) return 'yesterday'
    if (hours < 24 * 6) return d.toLocaleDateString('en-US', { weekday: 'short' })
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
  }
  const meta = (r: Recent) => r.missing ? 'Missing' :
    [r.pages != null && `${r.pages} ${r.pages === 1 ? 'page' : 'pages'}`, r.boards && r.boards > 1 && `${r.boards} boards`, `Edited ${when(r.time)}`].filter(Boolean).join(' · ')

  async function openDialog() {
    const f = await open({ filters: [{ name: 'Storyboarder', extensions: ['storyboarder'] }] })
    if (typeof f === 'string') await openProject(f)
  }
  async function importImages() {
    const f = await open({ multiple: true, filters: [{ name: 'Images', extensions: ['png', 'jpg', 'jpeg'] }] })
    if (Array.isArray(f) && f.length) { importFiles = f; app.sheet = 'new' }
  }
  function key(e: KeyboardEvent) {
    if ((e.target as HTMLElement).closest?.('input,textarea') || app.sheet) return
    if (e.metaKey && e.key === 'o') { e.preventDefault(); openDialog() }
    if (e.metaKey && e.key === 'n') { e.preventDefault(); app.sheet = 'new' }
    if (e.metaKey && e.key === ',') { e.preventDefault(); openSettings() }
    if (e.key === 'Enter' && selected) openProject(selected)
  }
</script>

<svelte:window onkeydown={key} />

<div class="welcome">
  <aside class="sidebar glass">
    <div class="top" data-tauri-drag-region></div>
    <div class="hdr small">Projects</div>
    <button class="item" class:on={filter === 'recent'} onclick={() => (filter = 'recent')}><History size={15} /> Recent</button>
    <button class="item" class:on={filter === 'all'} onclick={() => (filter = 'all')}><Library size={15} /> All Projects</button>
    <div class="hdr small">Locations</div>
    {#each LOCATIONS as l}
      <button class="item" class:on={filter === l.id} onclick={() => (filter = l.id)}><l.icon size={15} /> {l.name}</button>
    {/each}
  </aside>

  <header class="bar" data-tauri-drag-region>
    <div class="t" data-tauri-drag-region>{filter === 'recent' ? 'Recent' : filter === 'all' ? 'All Projects' : LOCATIONS.find((l) => l.id === filter)?.name}</div>
    <div class="right">
      <div class="capsule glass">
        <button class="icon-btn" class:on2={!listMode} onclick={() => (listMode = false)}><LayoutGrid size={16} /></button>
        <button class="icon-btn" class:on2={listMode} onclick={() => (listMode = true)}><List size={16} /></button>
      </div>
      <div class="capsule glass"><button class="icon-btn label" onclick={importImages}><ImagePlus size={16} /> Import Images…</button></div>
      <div class="capsule glass"><button class="icon-btn label" onclick={openDialog}><FolderOpen size={16} /> Open…</button></div>
      <label class="search glass"><Search size={14} /><input placeholder="Search" bind:value={query} /></label>
      <button class="new" onclick={() => (app.sheet = 'new')}>New Project</button>
    </div>
  </header>

  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <main onclick={(e) => { if (e.target === e.currentTarget) selected = '' }}>
    <div class="starts">
      <button class="start" onclick={() => (app.sheet = 'new')}>
        <span class="ic blue"><FilePlus2 size={18} /></span>
        <span><b>New Project</b><span class="small muted">Pages and boards, page size, reading direction</span></span>
      </button>
      <button class="start" onclick={openDialog}>
        <span class="ic"><FolderOpen size={18} /></span>
        <span><b>Open…</b><span class="small muted">Open a .storyboarder project from disk</span></span>
      </button>
      <button class="start" onclick={importImages}>
        <span class="ic"><ImagePlus size={18} /></span>
        <span><b>Import Images…</b><span class="small muted">Turn scans or sketches into pages</span></span>
      </button>
    </div>

    <div class="rh"><h2>Recent Projects</h2><span class="small muted">Sorted by Date Edited</span></div>
    {#if !shown.length}
      <p class="empty muted">{recents.length ? 'No projects here.' : 'No recent projects yet. Create one to get started.'}</p>
    {:else if listMode}
      <div class="list">
        {#each shown as r (r.filename)}
          <button class="lrow" class:on={selected === r.filename} onclick={() => (selected = r.filename)} ondblclick={() => !r.missing && openProject(r.filename)}>
            <span class="lt">{#if r.thumb}<img src={fileUrl(r.thumb)} alt="" />{/if}</span>
            <b>{r.title}</b><span class="muted small">{meta(r)}</span><span class="faint small path">{r.filename.replace(home, '~')}</span>
          </button>
        {/each}
      </div>
    {:else}
      <div class="grid">
        {#each shown as r (r.filename)}
          <button class="card" class:on={selected === r.filename} class:missing={r.missing} onclick={() => (selected = r.filename)} ondblclick={() => !r.missing && openProject(r.filename)}>
            <span class="thumb">
              <span class="stack"></span>
              {#if r.thumb}<img src={fileUrl(r.thumb)} alt="" />{:else}<span class="blankpg"></span>{/if}
            </span>
            <span class="name">{r.title}</span>
            <span class="small muted">{meta(r)}</span>
          </button>
        {/each}
      </div>
    {/if}
  </main>
</div>

{#if app.sheet === 'new'}
  <NewProject {importFiles} onclose={() => { app.sheet = null; importFiles = null }} />
{/if}

<style>
  .welcome { position: absolute; inset: 0; background: var(--sb-content); }
  .sidebar { position: absolute; left: 8px; top: 8px; bottom: 8px; width: 232px; border-radius: 16px; padding: 0 10px; }
  .top { height: 44px; }
  .hdr { color: var(--sb-label-2); font-weight: 600; padding: 12px 8px 6px; }
  .item { display: flex; align-items: center; gap: 8px; width: 100%; height: 30px; padding: 0 8px; border-radius: 8px; }
  .item :global(svg) { color: var(--sb-accent); }
  .item.on { background: var(--sb-control); }
  .bar { position: absolute; left: 248px; right: 0; top: 0; height: 60px; display: flex; align-items: center; justify-content: space-between; padding: 0 8px 0 8px; }
  .t { font-weight: 700; font-size: 15px; }
  .right { display: flex; gap: 10px; align-items: center; }
  .icon-btn.on2 { background: var(--sb-control); }
  .search { display: flex; align-items: center; gap: 6px; height: 40px; width: 200px; padding: 0 12px; border-radius: 20px; color: var(--sb-label-3); }
  .search input { border: 0; background: none; outline: none; flex: 1; min-width: 0; }
  .new { height: 40px; padding: 0 16px; border-radius: 20px; background: var(--sb-accent); color: #fff; font-weight: 500; }
  main { position: absolute; left: 248px; right: 0; top: 60px; bottom: 0; overflow-y: auto; padding: 20px 40px 40px; }
  .starts { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; }
  .start { display: flex; align-items: center; gap: 12px; height: 64px; padding: 0 14px; border-radius: 12px; background: var(--sb-group); box-shadow: inset 0 0 0 0.5px var(--sb-separator); text-align: left; }
  .start:hover { background: var(--sb-hover); }
  .start b { display: block; font-weight: 600; margin-bottom: 2px; }
  .ic { width: 36px; height: 36px; border-radius: 9px; background: var(--sb-control); display: flex; align-items: center; justify-content: center; color: var(--sb-accent); flex: none; }
  .ic.blue { background: var(--sb-accent); color: #fff; }
  .rh { display: flex; justify-content: space-between; align-items: baseline; margin: 32px 0 20px; }
  h2 { font-size: 15px; margin: 0; }
  .empty { padding: 40px 0; text-align: center; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 24px 24px; }
  .card { display: flex; flex-direction: column; align-items: center; gap: 4px; }
  .thumb { position: relative; width: 100%; height: 224px; border-radius: 12px; display: flex; align-items: center; justify-content: center; margin-bottom: 8px; }
  .card.on .thumb { background: var(--sb-control); }
  .thumb img, .blankpg, .stack { width: 138px; height: 194px; background: var(--sb-paper); box-shadow: 0 2px 6px #0006; position: relative; object-fit: cover; }
  .stack { position: absolute; transform: translate(5px, 5px); background: #e8e7e3; }
  .name { font-weight: 600; padding: 1px 8px; border-radius: 5px; }
  .card.on .name { background: var(--sb-selection); color: #fff; }
  .card.missing { opacity: 0.45; }
  .list { display: flex; flex-direction: column; }
  .lrow { display: grid; grid-template-columns: 36px 1fr 220px 1fr; align-items: center; gap: 12px; height: 52px; padding: 0 10px; border-radius: 8px; text-align: left; }
  .lrow.on { background: var(--sb-selection); }
  .lt img { width: 30px; height: 42px; object-fit: cover; display: block; }
  .path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
