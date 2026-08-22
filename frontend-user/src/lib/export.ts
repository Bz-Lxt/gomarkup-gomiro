import { contentBounds, shapeAABB, sortedShapes } from './geometry'
import { drawShape } from './render'
import type { Shape } from './protocol'
import { nowBeijing } from './time'

function live(shapes: Record<string, Shape>): Shape[] {
  return sortedShapes(Object.values(shapes))
}

export function exportJSON(boardId: string, shapes: Record<string, Shape>, groups: Record<string, string[]>): string {
  const items = live(shapes).map((s) => ({ ...s }))
  return JSON.stringify(
    {
      boardId,
      exportedAt: nowBeijing(),
      shapes: items,
      groups,
    },
    null,
    2,
  )
}

export function exportSVG(shapes: Record<string, Shape>): string {
  const list = live(shapes)
  const b = contentBounds(list)
  const w = Math.max(1, b.maxX - b.minX)
  const h = Math.max(1, b.maxY - b.minY)
  const parts = list.map((s) => shapeToSvg(s, b.minX, b.minY)).join('\n')
  return `<?xml version="1.0" encoding="UTF-8"?>\n<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" fill="none">\n${parts}\n</svg>`
}

function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

function dash(s: Shape): string {
  return s.dash === 'dashed' ? 'stroke-dasharray="8 6"' : ''
}

function shapeToSvg(s: Shape, ox: number, oy: number): string {
  const x = s.x - ox
  const y = s.y - oy
  const op = `opacity="${s.opacity}"`
  const st = `stroke="${s.stroke}" stroke-width="${s.strokeW}" ${dash(s)} ${op}`
  const fl = `fill="${s.fill === '#00000000' ? 'none' : s.fill}"`
  const rot = s.rotation ? ` transform="rotate(${s.rotation} ${x + s.w / 2} ${y + s.h / 2})"` : ''
  switch (s.kind) {
    case 'rect':
    case 'sticky':
    case 'image':
    case 'text':
      return `<g${rot}><rect x="${x}" y="${y}" width="${s.w}" height="${s.h}" rx="${s.radius || (s.kind === 'sticky' ? 4 : 0)}" ${fl} ${st}/>${textEl(s, x, y)}</g>`
    case 'ellipse':
      return `<ellipse cx="${x + s.w / 2}" cy="${y + s.h / 2}" rx="${s.w / 2}" ry="${s.h / 2}" ${fl} ${st}${rot}/>`
    case 'diamond': {
      const cx = x + s.w / 2
      const cy = y + s.h / 2
      return `<polygon points="${cx},${y} ${x + s.w},${cy} ${cx},${y + s.h} ${x},${cy}" ${fl} ${st}${rot}/>`
    }
    case 'line':
    case 'arrow':
    case 'freedraw': {
      const pts = (s.points ?? []).map((p) => `${p.x + x},${p.y + y}`).join(' ')
      const d = pts || `${x},${y} ${x + s.w},${y + s.h}`
      return `<polyline points="${d}" fill="none" ${st} stroke-linecap="round" stroke-linejoin="round"/>`
    }
    default:
      return ''
  }
}

function textEl(s: Shape, x: number, y: number): string {
  if (!s.text) return ''
  const ax = s.align === 'center' ? x + s.w / 2 : s.align === 'right' ? x + s.w - 8 : x + 8
  const anchor = s.align === 'center' ? 'middle' : s.align === 'right' ? 'end' : 'start'
  return `<text x="${ax}" y="${y + (s.fontSize || 16) + 8}" font-size="${s.fontSize || 16}" fill="${s.stroke}" text-anchor="${anchor}" font-family="Figtree, sans-serif">${esc(s.text)}</text>`
}

export function exportPNG(shapes: Record<string, Shape>, pixelRatio = 2): Promise<Blob> {
  const list = live(shapes)
  const b = contentBounds(list)
  const pad = 24
  const w = Math.max(1, b.maxX - b.minX + pad * 2)
  const h = Math.max(1, b.maxY - b.minY + pad * 2)
  const canvas = document.createElement('canvas')
  canvas.width = Math.ceil(w * pixelRatio)
  canvas.height = Math.ceil(h * pixelRatio)
  const ctx = canvas.getContext('2d')
  if (!ctx) return Promise.reject(new Error('canvas'))
  ctx.scale(pixelRatio, pixelRatio)
  ctx.fillStyle = '#f3eadc'
  ctx.fillRect(0, 0, w, h)
  ctx.translate(-b.minX + pad, -b.minY + pad)
  for (const s of list) drawShape(ctx, s)
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('toBlob'))), 'image/png')
  })
}

export function thumbnailDataURL(shapes: Record<string, Shape>, tw = 320, th = 180): string {
  const list = live(shapes)
  const b = contentBounds(list)
  const canvas = document.createElement('canvas')
  canvas.width = tw
  canvas.height = th
  const ctx = canvas.getContext('2d')
  if (!ctx) return ''
  ctx.fillStyle = '#211c16'
  ctx.fillRect(0, 0, tw, th)
  const bw = Math.max(1, b.maxX - b.minX)
  const bh = Math.max(1, b.maxY - b.minY)
  const scale = Math.min(tw / bw, th / bh) * 0.88
  ctx.translate(tw / 2, th / 2)
  ctx.scale(scale, scale)
  ctx.translate(-(b.minX + bw / 2), -(b.minY + bh / 2))
  for (const s of list) {
    if (shapeAABB(s).maxX - shapeAABB(s).minX > 0) drawShape(ctx, s)
  }
  return canvas.toDataURL('image/jpeg', 0.72)
}

export function downloadBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

export function downloadText(text: string, name: string, mime: string) {
  downloadBlob(new Blob([text], { type: mime }), name)
}
