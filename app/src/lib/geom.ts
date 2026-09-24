import type { Pt, Panel, Placement } from './sb.svelte'

export type Box = [number, number, number, number]

export function bbox(pts: Pt[]): Box {
  const xs = pts.map((p) => p[0]), ys = pts.map((p) => p[1])
  const x = Math.min(...xs), y = Math.min(...ys)
  return [x, y, Math.max(...xs) - x, Math.max(...ys) - y]
}

export function inPoly(p: Pt, poly: Pt[]) {
  let c = false
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const [xi, yi] = poly[i], [xj, yj] = poly[j]
    if (yi > p[1] !== yj > p[1] && p[0] < ((xj - xi) * (p[1] - yi)) / (yj - yi) + xi) c = !c
  }
  return c
}

export const ptsAttr = (pts: Pt[]) => pts.map((p) => p.join(',')).join(' ')
export const ptsArg = (pts: Pt[]) => pts.map((p) => `${Math.round(p[0])},${Math.round(p[1])}`).join(' ')

/** Maps points from box a to box b (resizing a panel by its bounding box). */
export function mapBox(pts: Pt[], a: Box, b: Box): Pt[] {
  return pts.map(([x, y]) => [b[0] + ((x - a[0]) * b[2]) / (a[2] || 1), b[1] + ((y - a[1]) * b[3]) / (a[3] || 1)])
}

/** Resizes box b by dragging handle (hx,hy in -1/0/1) by (dx,dy). */
export function resizeBox(b: Box, hx: number, hy: number, dx: number, dy: number, min = 20): Box {
  let [x, y, w, h] = b
  if (hx < 0) { x += dx; w -= dx } else if (hx > 0) w += dx
  if (hy < 0) { y += dy; h -= dy } else if (hy > 0) h += dy
  if (w < min) { if (hx < 0) x -= min - w; w = min }
  if (h < min) { if (hy < 0) y -= min - h; h = min }
  return [x, y, w, h]
}

export const HANDLES: [number, number][] = [[-1, -1], [0, -1], [1, -1], [1, 0], [1, 1], [0, 1], [-1, 1], [-1, 0]]
export const handlePos = (b: Box, [hx, hy]: [number, number]): Pt => [b[0] + ((hx + 1) / 2) * b[2], b[1] + ((hy + 1) / 2) * b[3]]

/** Placement of a board in a panel, as in manga-format.md: center + offset, fit × scale, rotation. */
export function placement(panel: Panel, c: Placement, size: Pt) {
  const [px, py, pw, ph] = panel.box
  const [bw, bh] = size
  const base = c.fit === 'fill' ? Math.max(pw / bw, ph / bh) : c.fit === 'fit' ? Math.min(pw / bw, ph / bh) : 1
  const s = base * c.scale
  const cx = px + pw / 2 + c.x, cy = py + ph / 2 + c.y
  return { cx, cy, s, w: bw * s, h: bh * s, rot: c.rotation, base }
}

export const rotate = ([x, y]: Pt, deg: number, [cx, cy]: Pt): Pt => {
  const r = (deg * Math.PI) / 180, dx = x - cx, dy = y - cy
  return [cx + dx * Math.cos(r) - dy * Math.sin(r), cy + dx * Math.sin(r) + dy * Math.cos(r)]
}
