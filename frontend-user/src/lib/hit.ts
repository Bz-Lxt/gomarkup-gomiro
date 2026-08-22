import { shapeAABB, pointInAABB } from './geometry'
import type { AABB } from './geometry'
import type { Shape } from './protocol'

export type HandleName = 'nw' | 'n' | 'ne' | 'e' | 'se' | 's' | 'sw' | 'w' | 'rot' | 'start' | 'end'

function distSeg(px: number, py: number, ax: number, ay: number, bx: number, by: number): number {
  const dx = bx - ax
  const dy = by - ay
  if (dx === 0 && dy === 0) return Math.hypot(px - ax, py - ay)
  let t = ((px - ax) * dx + (py - ay) * dy) / (dx * dx + dy * dy)
  t = Math.max(0, Math.min(1, t))
  return Math.hypot(px - (ax + t * dx), py - (ay + t * dy))
}

function inRotated(x: number, y: number, s: Shape): boolean {
  if (!s.rotation) return pointInAABB(x, y, shapeAABB(s))
  const cx = s.x + s.w / 2
  const cy = s.y + s.h / 2
  const rad = (-s.rotation * Math.PI) / 180
  const cos = Math.cos(rad)
  const sin = Math.sin(rad)
  const dx = x - cx
  const dy = y - cy
  const lx = dx * cos - dy * sin
  const ly = dx * sin + dy * cos
  return Math.abs(lx) <= Math.abs(s.w) / 2 && Math.abs(ly) <= Math.abs(s.h) / 2
}

function hitStroke(s: Shape, x: number, y: number, pad: number): boolean {
  const tol = s.strokeW / 2 + pad
  if (s.points && s.points.length >= 2) {
    for (let i = 1; i < s.points.length; i++) {
      const a = s.points[i - 1]
      const b = s.points[i]
      if (distSeg(x, y, a.x + s.x, a.y + s.y, b.x + s.x, b.y + s.y) <= tol) return true
    }
    return false
  }
  return distSeg(x, y, s.x, s.y, s.x + s.w, s.y + s.h) <= tol
}

export function hitTest(s: Shape, x: number, y: number, pad: number): boolean {
  if (s.deleted) return false
  switch (s.kind) {
    case 'ellipse': {
      if (!s.w || !s.h) return false
      const nx = (x - (s.x + s.w / 2)) / (s.w / 2)
      const ny = (y - (s.y + s.h / 2)) / (s.h / 2)
      return nx * nx + ny * ny <= 1
    }
    case 'diamond': {
      if (!s.w || !s.h) return false
      return Math.abs((x - (s.x + s.w / 2)) / (s.w / 2)) + Math.abs((y - (s.y + s.h / 2)) / (s.h / 2)) <= 1
    }
    case 'line':
    case 'arrow':
    case 'freedraw':
      return hitStroke(s, x, y, pad)
    default:
      return inRotated(x, y, s)
  }
}

export function pickTop(shapes: Shape[], x: number, y: number, pad: number): Shape | null {
  for (let i = shapes.length - 1; i >= 0; i--) {
    if (hitTest(shapes[i], x, y, pad)) return shapes[i]
  }
  return null
}

export function marqueeHits(shapes: Shape[], box: AABB): string[] {
  return shapes.filter((s) => !s.deleted && intersectsSafe(shapeAABB(s), box)).map((s) => s.id)
}

function intersectsSafe(a: AABB, b: AABB): boolean {
  return a.minX <= b.maxX && a.maxX >= b.minX && a.minY <= b.maxY && a.maxY >= b.minY
}

export function handleHits(
  s: Shape,
  sx: number,
  sy: number,
  toScreen: (x: number, y: number) => { x: number; y: number },
  rad = 8,
): HandleName | null {
  if (s.kind === 'line' || s.kind === 'arrow') {
    const a = s.points?.[0] ? { x: s.x + s.points[0].x, y: s.y + s.points[0].y } : { x: s.x, y: s.y }
    const b = s.points && s.points.length
      ? { x: s.x + s.points[s.points.length - 1].x, y: s.y + s.points[s.points.length - 1].y }
      : { x: s.x + s.w, y: s.y + s.h }
    const sa = toScreen(a.x, a.y)
    const sb = toScreen(b.x, b.y)
    if (Math.hypot(sx - sa.x, sy - sa.y) <= rad + 2) return 'start'
    if (Math.hypot(sx - sb.x, sy - sb.y) <= rad + 2) return 'end'
    return null
  }
  if (s.kind === 'freedraw') return null
  const pts: [HandleName, number, number][] = [
    ['nw', s.x, s.y],
    ['n', s.x + s.w / 2, s.y],
    ['ne', s.x + s.w, s.y],
    ['e', s.x + s.w, s.y + s.h / 2],
    ['se', s.x + s.w, s.y + s.h],
    ['s', s.x + s.w / 2, s.y + s.h],
    ['sw', s.x, s.y + s.h],
    ['w', s.x, s.y + s.h / 2],
  ]
  const cx = s.x + s.w / 2
  const cy = s.y + s.h / 2
  for (const [name, x, y] of pts) {
    const p = rotate(x, y, cx, cy, s.rotation || 0)
    const sp = toScreen(p.x, p.y)
    if (Math.hypot(sx - sp.x, sy - sp.y) <= rad) return name
  }
  const top = rotate(cx, s.y - 28, cx, cy, s.rotation || 0)
  const st = toScreen(top.x, top.y)
  if (Math.hypot(sx - st.x, sy - st.y) <= rad + 2) return 'rot'
  return null
}

function rotate(x: number, y: number, cx: number, cy: number, deg: number) {
  const rad = (deg * Math.PI) / 180
  const dx = x - cx
  const dy = y - cy
  return { x: cx + dx * Math.cos(rad) - dy * Math.sin(rad), y: cy + dx * Math.sin(rad) + dy * Math.cos(rad) }
}
