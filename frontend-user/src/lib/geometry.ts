import type { Point, Shape } from './protocol'

export type AABB = { minX: number; minY: number; maxX: number; maxY: number }
export type Viewport = { x: number; y: number; scale: number }
export type Guide = { kind: 'v' | 'h'; at: number; from: number; to: number }

export const MIN_SCALE = 0.1
export const MAX_SCALE = 5

export function emptyAABB(): AABB {
  return { minX: 1, minY: 1, maxX: -1, maxY: -1 }
}

export function aabbEmpty(a: AABB): boolean {
  return a.maxX < a.minX || a.maxY < a.minY
}

export function intersects(a: AABB, b: AABB): boolean {
  return a.minX <= b.maxX && a.maxX >= b.minX && a.minY <= b.maxY && a.maxY >= b.minY
}

export function union(a: AABB, b: AABB): AABB {
  return {
    minX: Math.min(a.minX, b.minX),
    minY: Math.min(a.minY, b.minY),
    maxX: Math.max(a.maxX, b.maxX),
    maxY: Math.max(a.maxY, b.maxY),
  }
}

export function inflate(a: AABB, pad: number): AABB {
  return { minX: a.minX - pad, minY: a.minY - pad, maxX: a.maxX + pad, maxY: a.maxY + pad }
}

export function pointInAABB(x: number, y: number, b: AABB): boolean {
  return x >= b.minX && x <= b.maxX && y >= b.minY && y <= b.maxY
}

export function worldToScreen(vx: number, vy: number, vp: Viewport): { x: number; y: number } {
  return { x: (vx - vp.x) * vp.scale, y: (vy - vp.y) * vp.scale }
}

export function screenToWorld(sx: number, sy: number, vp: Viewport): { x: number; y: number } {
  return { x: sx / vp.scale + vp.x, y: sy / vp.scale + vp.y }
}

export function zoomAt(vp: Viewport, sx: number, sy: number, next: number): Viewport {
  const scale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, next))
  const w = screenToWorld(sx, sy, vp)
  return { x: w.x - sx / scale, y: w.y - sy / scale, scale }
}

export function viewportAABB(vp: Viewport, w: number, h: number): AABB {
  const a = screenToWorld(0, 0, vp)
  const b = screenToWorld(w, h, vp)
  return { minX: a.x, minY: a.y, maxX: b.x, maxY: b.y }
}

export function shapeAABB(s: Shape): AABB {
  if (s.points && s.points.length && (s.kind === 'freedraw' || s.kind === 'line' || s.kind === 'arrow')) {
    let minX = s.points[0].x
    let minY = s.points[0].y
    let maxX = minX
    let maxY = minY
    for (const p of s.points) {
      minX = Math.min(minX, p.x)
      minY = Math.min(minY, p.y)
      maxX = Math.max(maxX, p.x)
      maxY = Math.max(maxY, p.y)
    }
    const pad = (s.strokeW || 0) + 8
    return { minX: minX + s.x - pad, minY: minY + s.y - pad, maxX: maxX + s.x + pad, maxY: maxY + s.y + pad }
  }
  let x1 = s.x
  let y1 = s.y
  let x2 = s.x + s.w
  let y2 = s.y + s.h
  if (s.w < 0) [x1, x2] = [x2, x1]
  if (s.h < 0) [y1, y2] = [y2, y1]
  if (!s.rotation) return { minX: x1, minY: y1, maxX: x2, maxY: y2 }
  const cx = (x1 + x2) / 2
  const cy = (y1 + y2) / 2
  const rad = (s.rotation * Math.PI) / 180
  const cos = Math.cos(rad)
  const sin = Math.sin(rad)
  const corners = [
    [x1, y1],
    [x2, y1],
    [x2, y2],
    [x1, y2],
  ]
  let minX = Infinity
  let minY = Infinity
  let maxX = -Infinity
  let maxY = -Infinity
  for (const [x, y] of corners) {
    const dx = x - cx
    const dy = y - cy
    const rx = cx + dx * cos - dy * sin
    const ry = cy + dx * sin + dy * cos
    minX = Math.min(minX, rx)
    minY = Math.min(minY, ry)
    maxX = Math.max(maxX, rx)
    maxY = Math.max(maxY, ry)
  }
  return { minX, minY, maxX, maxY }
}

export function contentBounds(shapes: Iterable<Shape>): AABB {
  let acc: AABB | null = null
  for (const s of shapes) {
    if (s.deleted) continue
    const b = shapeAABB(s)
    acc = acc ? union(acc, b) : b
  }
  return acc ?? { minX: -400, minY: -300, maxX: 400, maxY: 300 }
}

export function anchorPoint(s: Shape, name: string): { x: number; y: number } {
  const cx = s.x + s.w / 2
  const cy = s.y + s.h / 2
  switch (name) {
    case 'n':
      return { x: cx, y: s.y }
    case 's':
      return { x: cx, y: s.y + s.h }
    case 'w':
      return { x: s.x, y: cy }
    case 'e':
      return { x: s.x + s.w, y: cy }
    case 'nw':
      return { x: s.x, y: s.y }
    case 'ne':
      return { x: s.x + s.w, y: s.y }
    case 'sw':
      return { x: s.x, y: s.y + s.h }
    case 'se':
      return { x: s.x + s.w, y: s.y + s.h }
    default:
      return { x: cx, y: cy }
  }
}

export const ANCHORS = ['n', 's', 'w', 'e', 'nw', 'ne', 'sw', 'se', 'c'] as const

export function nearestAnchor(s: Shape, x: number, y: number): string {
  let best = 'c'
  let bestD = Infinity
  for (const n of ANCHORS) {
    const p = anchorPoint(s, n)
    const d = Math.hypot(x - p.x, y - p.y)
    if (d < bestD) {
      bestD = d
      best = n
    }
  }
  return best
}

export function snapThreshold(scale: number): number {
  if (scale <= 0) return 8
  return Math.min(24, Math.max(2, 8 / scale))
}

export function snapMove(moving: AABB, others: AABB[], threshold: number): { x: number; y: number; guides: Guide[]; snapped: boolean } {
  const th = threshold > 0 ? threshold : 8
  const res = { x: 0, y: 0, guides: [] as Guide[], snapped: false }
  let bestX = th + 1
  let bestY = th + 1
  const mx = [moving.minX, (moving.minX + moving.maxX) / 2, moving.maxX]
  const my = [moving.minY, (moving.minY + moving.maxY) / 2, moving.maxY]
  for (const o of others) {
    const ox = [o.minX, (o.minX + o.maxX) / 2, o.maxX]
    const oy = [o.minY, (o.minY + o.maxY) / 2, o.maxY]
    for (const a of mx) {
      for (const b of ox) {
        const d = b - a
        const ad = Math.abs(d)
        if (ad < bestX && ad <= th) {
          bestX = ad
          res.x = d
          res.guides = res.guides.filter((g) => g.kind !== 'v')
          res.guides.push({ kind: 'v', at: b, from: Math.min(moving.minY, o.minY), to: Math.max(moving.maxY, o.maxY) })
        }
      }
    }
    for (const a of my) {
      for (const b of oy) {
        const d = b - a
        const ad = Math.abs(d)
        if (ad < bestY && ad <= th) {
          bestY = ad
          res.y = d
          res.guides = res.guides.filter((g) => g.kind !== 'h')
          res.guides.push({ kind: 'h', at: b, from: Math.min(moving.minX, o.minX), to: Math.max(moving.maxX, o.maxX) })
        }
      }
    }
  }
  res.snapped = bestX <= th || bestY <= th
  if (bestX > th) res.x = 0
  if (bestY > th) res.y = 0
  return res
}

export function rotateAround(x: number, y: number, cx: number, cy: number, deg: number): { x: number; y: number } {
  const rad = (deg * Math.PI) / 180
  const cos = Math.cos(rad)
  const sin = Math.sin(rad)
  const dx = x - cx
  const dy = y - cy
  return { x: cx + dx * cos - dy * sin, y: cy + dx * sin + dy * cos }
}

export function toLocal(x: number, y: number, s: Shape): { x: number; y: number } {
  const cx = s.x + s.w / 2
  const cy = s.y + s.h / 2
  const p = rotateAround(x, y, cx, cy, -(s.rotation || 0))
  return { x: p.x - s.x, y: p.y - s.y }
}

export function nextZ(shapes: Iterable<Shape>): number {
  let max = 0
  for (const s of shapes) {
    if (!s.deleted && s.z > max) max = s.z
  }
  return max + 1
}

export function sortedShapes(shapes: Iterable<Shape>): Shape[] {
  return [...shapes].filter((s) => !s.deleted).sort((a, b) => (a.z === b.z ? a.id.localeCompare(b.id) : a.z - b.z))
}

export function gridSpec(scale: number): { step: number; mode: 'dot' | 'line' } {
  if (scale >= 1.15) return { step: 20, mode: 'line' }
  if (scale >= 0.55) return { step: 40, mode: 'line' }
  if (scale >= 0.28) return { step: 80, mode: 'dot' }
  return { step: 160, mode: 'dot' }
}

export function worldPoints(s: Shape): Point[] {
  if (s.points && s.points.length) {
    return s.points.map((p) => ({ x: p.x + s.x, y: p.y + s.y, p: p.p }))
  }
  return [
    { x: s.x, y: s.y },
    { x: s.x + s.w, y: s.y + s.h },
  ]
}

export function clamp(n: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, n))
}
