import { gridSpec, shapeAABB, viewportAABB, worldToScreen, type AABB, type Guide, type Viewport } from './geometry'
import type { Member, Shape } from './protocol'

const imgCache = new Map<string, HTMLImageElement>()
const pending = new Set<string>()

export function primeImage(url: string, onLoad?: () => void): HTMLImageElement | null {
  if (!url) return null
  let img = imgCache.get(url)
  if (img?.complete && img.naturalWidth) return img
  if (!img) {
    img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      pending.delete(url)
      onLoad?.()
    }
    img.src = url
    imgCache.set(url, img)
    pending.add(url)
  }
  return img.complete && img.naturalWidth ? img : null
}

export function resizeCanvas(el: HTMLCanvasElement, cssW: number, cssH: number): CanvasRenderingContext2D | null {
  const dpr = Math.min(window.devicePixelRatio || 1, 3)
  const w = Math.max(1, Math.floor(cssW * dpr))
  const h = Math.max(1, Math.floor(cssH * dpr))
  if (el.width !== w || el.height !== h) {
    el.width = w
    el.height = h
    el.style.width = `${cssW}px`
    el.style.height = `${cssH}px`
  }
  const ctx = el.getContext('2d')
  if (!ctx) return null
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  return ctx
}

export function applyWorld(ctx: CanvasRenderingContext2D, vp: Viewport) {
  ctx.setTransform(1, 0, 0, 1, 0, 0)
  const dpr = Math.min(window.devicePixelRatio || 1, 3)
  ctx.setTransform(dpr * vp.scale, 0, 0, dpr * vp.scale, -vp.x * dpr * vp.scale, -vp.y * dpr * vp.scale)
}

export function drawGrid(ctx: CanvasRenderingContext2D, vp: Viewport, cssW: number, cssH: number, dark: boolean) {
  const { step, mode } = gridSpec(vp.scale)
  const view = viewportAABB(vp, cssW, cssH)
  const x0 = Math.floor(view.minX / step) * step
  const y0 = Math.floor(view.minY / step) * step
  ctx.save()
  if (mode === 'line') {
    ctx.strokeStyle = dark ? 'rgba(243,234,220,0.06)' : 'rgba(28,25,21,0.08)'
    ctx.lineWidth = 1 / vp.scale
    ctx.beginPath()
    for (let x = x0; x <= view.maxX; x += step) {
      ctx.moveTo(x, view.minY)
      ctx.lineTo(x, view.maxY)
    }
    for (let y = y0; y <= view.maxY; y += step) {
      ctx.moveTo(view.minX, y)
      ctx.lineTo(view.maxX, y)
    }
    ctx.stroke()
  } else {
    ctx.fillStyle = dark ? 'rgba(196,163,90,0.28)' : 'rgba(47,93,86,0.28)'
    const r = 1.1 / vp.scale
    for (let x = x0; x <= view.maxX; x += step) {
      for (let y = y0; y <= view.maxY; y += step) {
        ctx.beginPath()
        ctx.arc(x, y, r, 0, Math.PI * 2)
        ctx.fill()
      }
    }
  }
  ctx.restore()
}

function strokeStyle(ctx: CanvasRenderingContext2D, s: Shape) {
  ctx.globalAlpha = s.opacity
  ctx.strokeStyle = s.stroke
  ctx.lineWidth = s.strokeW || 1.5
  ctx.setLineDash(s.dash === 'dashed' ? [8, 6] : [])
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
}

function fillOf(s: Shape): string | null {
  if (!s.fill || s.fill === '#00000000' || s.fill === 'transparent') return null
  return s.fill
}

function withRot(ctx: CanvasRenderingContext2D, s: Shape, fn: () => void) {
  if (!s.rotation) {
    fn()
    return
  }
  ctx.save()
  ctx.translate(s.x + s.w / 2, s.y + s.h / 2)
  ctx.rotate((s.rotation * Math.PI) / 180)
  ctx.translate(-(s.x + s.w / 2), -(s.y + s.h / 2))
  fn()
  ctx.restore()
}

function wrapText(ctx: CanvasRenderingContext2D, text: string, maxW: number): string[] {
  const lines: string[] = []
  for (const para of text.split('\n')) {
    let cur = ''
    for (const ch of para) {
      const next = cur + ch
      if (ctx.measureText(next).width > maxW && cur) {
        lines.push(cur)
        cur = ch
      } else cur = next
    }
    lines.push(cur)
  }
  return lines
}

function drawText(ctx: CanvasRenderingContext2D, s: Shape, pad = 8) {
  if (!s.text) return
  ctx.fillStyle = s.stroke
  ctx.font = `${s.fontSize || 16}px Figtree, sans-serif`
  ctx.textBaseline = 'top'
  const lines = wrapText(ctx, s.text, Math.max(8, s.w - pad * 2))
  const lh = (s.fontSize || 16) * 1.35
  let y = s.y + pad
  for (const line of lines) {
    const w = ctx.measureText(line).width
    let x = s.x + pad
    if (s.align === 'center') x = s.x + (s.w - w) / 2
    if (s.align === 'right') x = s.x + s.w - pad - w
    ctx.fillText(line, x, y)
    y += lh
  }
}

export function drawShape(ctx: CanvasRenderingContext2D, s: Shape, onImg?: () => void) {
  ctx.save()
  strokeStyle(ctx, s)
  const fill = fillOf(s)
  withRot(ctx, s, () => {
    switch (s.kind) {
      case 'rect':
      case 'sticky': {
        const r = s.radius || (s.kind === 'sticky' ? 6 : 0)
        ctx.beginPath()
        if (typeof ctx.roundRect === 'function') ctx.roundRect(s.x, s.y, s.w, s.h, r)
        else ctx.rect(s.x, s.y, s.w, s.h)
        if (fill) {
          ctx.fillStyle = fill
          ctx.fill()
        }
        ctx.stroke()
        drawText(ctx, s, s.kind === 'sticky' ? 12 : 8)
        break
      }
      case 'ellipse':
        ctx.beginPath()
        ctx.ellipse(s.x + s.w / 2, s.y + s.h / 2, Math.abs(s.w / 2), Math.abs(s.h / 2), 0, 0, Math.PI * 2)
        if (fill) {
          ctx.fillStyle = fill
          ctx.fill()
        }
        ctx.stroke()
        break
      case 'diamond':
        ctx.beginPath()
        ctx.moveTo(s.x + s.w / 2, s.y)
        ctx.lineTo(s.x + s.w, s.y + s.h / 2)
        ctx.lineTo(s.x + s.w / 2, s.y + s.h)
        ctx.lineTo(s.x, s.y + s.h / 2)
        ctx.closePath()
        if (fill) {
          ctx.fillStyle = fill
          ctx.fill()
        }
        ctx.stroke()
        break
      case 'line':
      case 'arrow':
      case 'freedraw': {
        const pts = s.points && s.points.length >= 2 ? s.points : [
          { x: 0, y: 0 },
          { x: s.w, y: s.h },
        ]
        ctx.beginPath()
        ctx.moveTo(s.x + pts[0].x, s.y + pts[0].y)
        for (let i = 1; i < pts.length; i++) {
          const p = pts[i]
          if (s.kind === 'freedraw') {
            const w = (s.strokeW || 2) * (p.p || 1)
            ctx.lineWidth = w
          }
          ctx.lineTo(s.x + p.x, s.y + p.y)
        }
        ctx.stroke()
        if (s.kind === 'arrow') {
          const a = pts[pts.length - 2]
          const b = pts[pts.length - 1]
          const ax = s.x + a.x
          const ay = s.y + a.y
          const bx = s.x + b.x
          const by = s.y + b.y
          const ang = Math.atan2(by - ay, bx - ax)
          const len = 14 + (s.strokeW || 2)
          ctx.beginPath()
          ctx.moveTo(bx, by)
          ctx.lineTo(bx - Math.cos(ang - 0.4) * len, by - Math.sin(ang - 0.4) * len)
          ctx.moveTo(bx, by)
          ctx.lineTo(bx - Math.cos(ang + 0.4) * len, by - Math.sin(ang + 0.4) * len)
          ctx.stroke()
        }
        break
      }
      case 'text':
        if (fill) {
          ctx.fillStyle = fill
          ctx.fillRect(s.x, s.y, s.w, s.h)
        }
        drawText(ctx, s)
        break
      case 'image': {
        const img = primeImage(s.imageUrl || '', onImg)
        if (img) ctx.drawImage(img, s.x, s.y, s.w, s.h)
        else {
          ctx.fillStyle = fill || '#211c16'
          ctx.fillRect(s.x, s.y, s.w, s.h)
          ctx.stroke()
        }
        break
      }
    }
  })
  ctx.restore()
}

export function drawStatic(
  ctx: CanvasRenderingContext2D,
  shapes: Shape[],
  vp: Viewport,
  cssW: number,
  cssH: number,
  dark: boolean,
  onImg?: () => void,
) {
  ctx.setTransform(1, 0, 0, 1, 0, 0)
  const dpr = Math.min(window.devicePixelRatio || 1, 3)
  ctx.clearRect(0, 0, cssW * dpr, cssH * dpr)
  applyWorld(ctx, vp)
  drawGrid(ctx, vp, cssW, cssH, dark)
  const view = inflateView(viewportAABB(vp, cssW, cssH), 80)
  for (const s of shapes) {
    if (s.deleted) continue
    const box = shapeAABB(s)
    if (box.maxX < view.minX || box.minX > view.maxX || box.maxY < view.minY || box.minY > view.maxY) continue
    drawShape(ctx, s, onImg)
  }
}

function inflateView(a: AABB, pad: number): AABB {
  return { minX: a.minX - pad, minY: a.minY - pad, maxX: a.maxX + pad, maxY: a.maxY + pad }
}

export function drawOverlay(opts: {
  ctx: CanvasRenderingContext2D
  vp: Viewport
  cssW: number
  cssH: number
  selected: Shape[]
  remote: { color: string; shapes: Shape[] }[]
  cursors: { x: number; y: number; color: string; name: string }[]
  guides: Guide[]
  marquee: AABB | null
  preview: Shape | null
  selfId?: string
}) {
  const { ctx, vp, cssW, cssH } = opts
  ctx.setTransform(1, 0, 0, 1, 0, 0)
  const dpr = Math.min(window.devicePixelRatio || 1, 3)
  ctx.clearRect(0, 0, cssW * dpr, cssH * dpr)
  applyWorld(ctx, vp)

  if (opts.preview) drawShape(ctx, opts.preview)

  for (const r of opts.remote) {
    ctx.save()
    ctx.strokeStyle = r.color
    ctx.setLineDash([6 / vp.scale, 5 / vp.scale])
    ctx.lineWidth = 1.6 / vp.scale
    for (const s of r.shapes) strokeBox(ctx, s)
    ctx.restore()
  }

  ctx.save()
  ctx.strokeStyle = '#c4a35a'
  ctx.lineWidth = 1.5 / vp.scale
  ctx.setLineDash([])
  for (const s of opts.selected) {
    strokeBox(ctx, s)
    drawHandles(ctx, s, vp.scale)
  }
  ctx.restore()

  if (opts.guides.length) {
    ctx.save()
    ctx.strokeStyle = '#c4a35a'
    ctx.lineWidth = 1 / vp.scale
    ctx.setLineDash([4 / vp.scale, 3 / vp.scale])
    ctx.beginPath()
    for (const g of opts.guides) {
      if (g.kind === 'v') {
        ctx.moveTo(g.at, g.from)
        ctx.lineTo(g.at, g.to)
      } else {
        ctx.moveTo(g.from, g.at)
        ctx.lineTo(g.to, g.at)
      }
    }
    ctx.stroke()
    ctx.restore()
  }

  if (opts.marquee && opts.marquee.maxX > opts.marquee.minX) {
    ctx.save()
    ctx.strokeStyle = '#2f5d56'
    ctx.fillStyle = 'rgba(47,93,86,0.12)'
    ctx.lineWidth = 1 / vp.scale
    ctx.setLineDash([4 / vp.scale, 3 / vp.scale])
    const m = opts.marquee
    ctx.fillRect(m.minX, m.minY, m.maxX - m.minX, m.maxY - m.minY)
    ctx.strokeRect(m.minX, m.minY, m.maxX - m.minX, m.maxY - m.minY)
    ctx.restore()
  }

  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  for (const c of opts.cursors) {
    const p = worldToScreen(c.x, c.y, vp)
    drawCursor(ctx, p.x, p.y, c.color, c.name)
  }
}

function strokeBox(ctx: CanvasRenderingContext2D, s: Shape) {
  const b = shapeAABB(s)
  ctx.strokeRect(b.minX, b.minY, b.maxX - b.minX, b.maxY - b.minY)
}

function drawHandles(ctx: CanvasRenderingContext2D, s: Shape, scale: number) {
  const r = 4 / scale
  const dots: { x: number; y: number }[] = []
  if (s.kind === 'line' || s.kind === 'arrow') {
    const a = s.points?.[0] ? { x: s.x + s.points[0].x, y: s.y + s.points[0].y } : { x: s.x, y: s.y }
    const b = s.points?.length
      ? { x: s.x + s.points[s.points.length - 1].x, y: s.y + s.points[s.points.length - 1].y }
      : { x: s.x + s.w, y: s.y + s.h }
    dots.push(a, b)
  } else if (s.kind !== 'freedraw') {
    const raw = [
      [s.x, s.y],
      [s.x + s.w / 2, s.y],
      [s.x + s.w, s.y],
      [s.x + s.w, s.y + s.h / 2],
      [s.x + s.w, s.y + s.h],
      [s.x + s.w / 2, s.y + s.h],
      [s.x, s.y + s.h],
      [s.x, s.y + s.h / 2],
    ]
    const cx = s.x + s.w / 2
    const cy = s.y + s.h / 2
    const rad = ((s.rotation || 0) * Math.PI) / 180
    for (const [x, y] of raw) {
      const dx = x - cx
      const dy = y - cy
      dots.push({ x: cx + dx * Math.cos(rad) - dy * Math.sin(rad), y: cy + dx * Math.sin(rad) + dy * Math.cos(rad) })
    }
    const top = {
      x: cx + (0) * Math.cos(rad) - (-28) * Math.sin(rad),
      y: cy + (0) * Math.sin(rad) + (-28) * Math.cos(rad),
    }
    ctx.beginPath()
    ctx.moveTo(cx + (0) * Math.cos(rad) - ((-s.h / 2) * Math.sin(rad)), cy + 0)
    const mid = { x: cx, y: s.y }
    const midR = {
      x: cx + (mid.x - cx) * Math.cos(rad) - (mid.y - cy) * Math.sin(rad),
      y: cy + (mid.x - cx) * Math.sin(rad) + (mid.y - cy) * Math.cos(rad),
    }
    ctx.beginPath()
    ctx.moveTo(midR.x, midR.y)
    ctx.lineTo(top.x, top.y)
    ctx.stroke()
    dots.push(top)
  }
  ctx.fillStyle = '#f3eadc'
  ctx.strokeStyle = '#c4a35a'
  ctx.lineWidth = 1.2 / scale
  ctx.setLineDash([])
  for (const d of dots) {
    ctx.beginPath()
    ctx.rect(d.x - r, d.y - r, r * 2, r * 2)
    ctx.fill()
    ctx.stroke()
  }
}

function drawCursor(ctx: CanvasRenderingContext2D, x: number, y: number, color: string, name: string) {
  ctx.save()
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.moveTo(x, y)
  ctx.lineTo(x + 12, y + 18)
  ctx.lineTo(x + 6, y + 17)
  ctx.lineTo(x + 9, y + 26)
  ctx.lineTo(x + 5, y + 27)
  ctx.lineTo(x + 2, y + 18)
  ctx.closePath()
  ctx.fill()
  ctx.font = '600 11px Figtree, sans-serif'
  const w = ctx.measureText(name).width + 10
  ctx.fillRect(x + 14, y + 6, w, 18)
  ctx.fillStyle = '#f3eadc'
  ctx.fillText(name, x + 19, y + 19)
  ctx.restore()
}

export function drawMinimap(
  ctx: CanvasRenderingContext2D,
  shapes: Shape[],
  vp: Viewport,
  cssW: number,
  cssH: number,
  viewW: number,
  viewH: number,
) {
  const dpr = Math.min(window.devicePixelRatio || 1, 3)
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, cssW, cssH)
  ctx.fillStyle = 'rgba(22,20,17,0.55)'
  ctx.fillRect(0, 0, cssW, cssH)
  const b = (() => {
    let acc: AABB | null = null
    for (const s of shapes) {
      if (s.deleted) continue
      const box = shapeAABB(s)
      acc = acc
        ? { minX: Math.min(acc.minX, box.minX), minY: Math.min(acc.minY, box.minY), maxX: Math.max(acc.maxX, box.maxX), maxY: Math.max(acc.maxY, box.maxY) }
        : box
    }
    return acc ?? { minX: -200, minY: -150, maxX: 200, maxY: 150 }
  })()
  const bw = Math.max(40, b.maxX - b.minX)
  const bh = Math.max(40, b.maxY - b.minY)
  const scale = Math.min((cssW - 12) / bw, (cssH - 12) / bh)
  const ox = (cssW - bw * scale) / 2 - b.minX * scale
  const oy = (cssH - bh * scale) / 2 - b.minY * scale
  ctx.save()
  ctx.translate(ox, oy)
  ctx.scale(scale, scale)
  ctx.fillStyle = '#c4a35a'
  for (const s of shapes) {
    if (s.deleted) continue
    const box = shapeAABB(s)
    ctx.globalAlpha = 0.55
    ctx.fillRect(box.minX, box.minY, Math.max(2, box.maxX - box.minX), Math.max(2, box.maxY - box.minY))
  }
  const view = viewportAABB(vp, viewW, viewH)
  ctx.globalAlpha = 1
  ctx.strokeStyle = '#9b2c2c'
  ctx.lineWidth = 2 / scale
  ctx.strokeRect(view.minX, view.minY, view.maxX - view.minX, view.maxY - view.minY)
  ctx.restore()
  return { scale, ox, oy }
}

export type MiniMapXform = { scale: number; ox: number; oy: number }

export function memberCursorLabel(m: Member): string {
  return m.nickname || 'Guest'
}
