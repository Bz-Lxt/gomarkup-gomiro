import { onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { contentBounds, nearestAnchor, screenToWorld, shapeAABB, snapMove, snapThreshold, worldToScreen, zoomAt, type AABB, type Viewport } from '@/lib/geometry'
import { handleHits, marqueeHits, pickTop, type HandleName } from '@/lib/hit'
import { drawOverlay, drawStatic, resizeCanvas } from '@/lib/render'
import { undoStack } from '@/lib/undo'
import { cloneShape, type Shape, type ShapeKind } from '@/lib/protocol'
import { makeShape, useDocStore } from '@/stores/doc'
import { useSessionStore } from '@/stores/session'
import { useUiStore, type Tool } from '@/stores/ui'

type Mode =
  | { t: 'idle' }
  | { t: 'pan'; sx: number; sy: number; vx: number; vy: number }
  | { t: 'draw'; kind: ShapeKind; x0: number; y0: number }
  | { t: 'pen'; id: string }
  | { t: 'move'; ids: string[]; x0: number; y0: number; starts: Record<string, Shape> }
  | { t: 'resize'; id: string; handle: HandleName; start: Shape }
  | { t: 'rotate'; id: string; start: Shape }
  | { t: 'marquee'; x0: number; y0: number }

const DRAW_TOOLS: Tool[] = ['rect', 'ellipse', 'diamond', 'line', 'arrow', 'freedraw', 'text', 'sticky']

function isTypingTarget(el: EventTarget | null): boolean {
  const n = el as HTMLElement | null
  if (!n) return false
  const tag = n.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || n.isContentEditable
}

export function useCanvas(wrap: Ref<HTMLElement | null>, staticEl: Ref<HTMLCanvasElement | null>, overlayEl: Ref<HTMLCanvasElement | null>) {
  const doc = useDocStore()
  const ui = useUiStore()
  const session = useSessionStore()
  const size = ref({ w: 0, h: 0 })
  let raf = 0
  let mode: Mode = { t: 'idle' }
  let ro: ResizeObserver | null = null

  function vp(): Viewport {
    return ui.viewport
  }

  function localXY(e: PointerEvent): { sx: number; sy: number; wx: number; wy: number } {
    const r = wrap.value!.getBoundingClientRect()
    const sx = e.clientX - r.left
    const sy = e.clientY - r.top
    const w = screenToWorld(sx, sy, vp())
    return { sx, sy, wx: w.x, wy: w.y }
  }

  function loop() {
    const st = staticEl.value
    const ov = overlayEl.value
    if (st && ov && wrap.value) {
      const r = wrap.value.getBoundingClientRect()
      size.value = { w: r.width, h: r.height }
      if (doc.dirtyStatic) {
        const ctx = resizeCanvas(st, r.width, r.height)
        if (ctx) {
          drawStatic(ctx, doc.liveList, vp(), r.width, r.height, session.theme === 'dark', () => {
            doc.markStatic()
          })
          doc.dirtyStatic = false
        }
      }
      if (doc.dirtyOverlay) {
        const ctx = resizeCanvas(ov, r.width, r.height)
        if (ctx) {
          const selected = ui.selectedIds.map((id) => doc.shapes[id]).filter(Boolean)
          const remote = Object.entries(session.remoteSel).map(([cid, ids]) => {
            const m = session.members.find((x) => x.id === cid)
            return {
              color: m?.color || '#c4a35a',
              shapes: ids.map((id) => doc.shapes[id]).filter(Boolean) as Shape[],
            }
          })
          const cursors = Object.entries(session.cursors).map(([idx, p]) => {
            const m = session.members.find((x) => x.userIdx === Number(idx))
            return { x: p.x, y: p.y, color: m?.color || '#c4a35a', name: m?.nickname || 'Guest' }
          })
          drawOverlay({
            ctx,
            vp: vp(),
            cssW: r.width,
            cssH: r.height,
            selected,
            remote,
            cursors,
            guides: ui.guides,
            marquee: ui.marquee,
            preview: ui.preview,
          })
          doc.dirtyOverlay = false
        }
      }
    }
    raf = requestAnimationFrame(loop)
  }

  function attachArrow(kind: ShapeKind, x: number, y: number, skip?: string) {
    if (kind !== 'arrow' && kind !== 'line') return {}
    const pad = snapThreshold(vp().scale) * 1.4
    let best: { id: string; anchor: string; d: number } | null = null
    for (const s of doc.liveList) {
      if (s.id === skip || s.kind === 'line' || s.kind === 'arrow' || s.kind === 'freedraw') continue
      const a = nearestAnchor(s, x, y)
      const ap = ((): { x: number; y: number } => {
        const cx = s.x + s.w / 2
        const cy = s.y + s.h / 2
        switch (a) {
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
      })()
      const d = Math.hypot(x - ap.x, y - ap.y)
      if (d <= pad && (!best || d < best.d)) best = { id: s.id, anchor: a, d }
    }
    return best ? { id: best.id, anchor: best.anchor, } : {}
  }

  function onPointerDown(e: PointerEvent) {
    if (!wrap.value || ui.editingTextId) return
    wrap.value.setPointerCapture(e.pointerId)
    const p = localXY(e)
    const space = ui.spaceDown || e.button === 1 || e.buttons === 4
    if (space || e.button === 1) {
      mode = { t: 'pan', sx: p.sx, sy: p.sy, vx: vp().x, vy: vp().y }
      return
    }
    if (e.button !== 0) return

    if (ui.tool === 'select') {
      const sel = ui.selectedIds.map((id) => doc.shapes[id]).filter(Boolean) as Shape[]
      if (sel.length === 1) {
        const h = handleHits(sel[0], p.sx, p.sy, (x, y) => worldToScreen(x, y, vp()))
        if (h === 'rot') {
          mode = { t: 'rotate', id: sel[0].id, start: cloneShape(sel[0]) }
          return
        }
        if (h) {
          mode = { t: 'resize', id: sel[0].id, handle: h, start: cloneShape(sel[0]) }
          return
        }
      }
      const hit = pickTop(doc.liveList, p.wx, p.wy, 6 / vp().scale)
      if (hit) {
        let ids = e.shiftKey ? [...ui.selectedIds] : []
        if (e.shiftKey) {
          ui.select([hit.id], true)
          ids = ui.selectedIds
        } else if (!ui.selectedIds.includes(hit.id)) {
          ids = doc.expandGroup([hit.id])
          ui.select(ids)
        } else {
          ids = ui.selectedIds
        }
        session.sendSelection(ui.selectedIds)
        const starts: Record<string, Shape> = {}
        for (const id of ids) if (doc.shapes[id]) starts[id] = cloneShape(doc.shapes[id])
        mode = { t: 'move', ids, x0: p.wx, y0: p.wy, starts }
      } else {
        if (!e.shiftKey) {
          ui.select([])
          session.sendSelection([])
        }
        mode = { t: 'marquee', x0: p.wx, y0: p.wy }
        ui.marquee = { minX: p.wx, minY: p.wy, maxX: p.wx, maxY: p.wy }
      }
      doc.markOverlay()
      return
    }

    if (ui.tool === 'freedraw') {
      const s = makeShape('freedraw', session.theme === 'dark', {
        x: p.wx,
        y: p.wy,
        w: 0,
        h: 0,
        points: [{ x: 0, y: 0, p: 1 }],
      })
      ui.preview = s
      mode = { t: 'pen', id: s.id }
      doc.markOverlay()
      return
    }

    if (DRAW_TOOLS.includes(ui.tool)) {
      mode = { t: 'draw', kind: ui.tool as ShapeKind, x0: p.wx, y0: p.wy }
      ui.preview = makeShape(ui.tool as ShapeKind, session.theme === 'dark', { x: p.wx, y: p.wy, w: 1, h: 1 })
      doc.markOverlay()
    }
  }

  function onPointerMove(e: PointerEvent) {
    if (!wrap.value) return
    const p = localXY(e)
    session.sendCursor(p.wx, p.wy)

    if (mode.t === 'idle') return
    if (mode.t === 'pan') {
      const dx = (p.sx - mode.sx) / vp().scale
      const dy = (p.sy - mode.sy) / vp().scale
      ui.viewport = { ...vp(), x: mode.vx - dx, y: mode.vy - dy }
      doc.markStatic()
      return
    }
    if (mode.t === 'draw') {
      const x = Math.min(mode.x0, p.wx)
      const y = Math.min(mode.y0, p.wy)
      const w = Math.abs(p.wx - mode.x0)
      const h = Math.abs(p.wy - mode.y0)
      if (mode.kind === 'line' || mode.kind === 'arrow') {
        const start = attachArrow(mode.kind, mode.x0, mode.y0)
        const end = attachArrow(mode.kind, p.wx, p.wy)
        ui.preview = makeShape(mode.kind, session.theme === 'dark', {
          ...(ui.preview || {}),
          x: mode.x0,
          y: mode.y0,
          w: p.wx - mode.x0,
          h: p.wy - mode.y0,
          points: [
            { x: 0, y: 0 },
            { x: p.wx - mode.x0, y: p.wy - mode.y0 },
          ],
          startId: start.id,
          startAnchor: start.anchor,
          endId: end.id,
          endAnchor: end.anchor,
        })
      } else {
        ui.preview = makeShape(mode.kind, session.theme === 'dark', {
          ...(ui.preview || {}),
          x,
          y,
          w: Math.max(1, w),
          h: Math.max(1, h),
        })
      }
      doc.markOverlay()
      return
    }
    if (mode.t === 'pen' && ui.preview) {
      const pts = ui.preview.points ? [...ui.preview.points] : []
      pts.push({ x: p.wx - ui.preview.x, y: p.wy - ui.preview.y, p: e.pressure || 1 })
      ui.preview = { ...ui.preview, points: pts }
      doc.markOverlay()
      return
    }
    if (mode.t === 'move') {
      let dx = p.wx - mode.x0
      let dy = p.wy - mode.y0
      const movingIds = new Set(mode.ids)
      let box: AABB | null = null
      for (const id of mode.ids) {
        const st = mode.starts[id]
        if (!st) continue
        const b = shapeAABB({ ...st, x: st.x + dx, y: st.y + dy })
        box = box
          ? { minX: Math.min(box.minX, b.minX), minY: Math.min(box.minY, b.minY), maxX: Math.max(box.maxX, b.maxX), maxY: Math.max(box.maxY, b.maxY) }
          : b
      }
      if (box) {
        const others = doc.liveList.filter((s) => !movingIds.has(s.id)).map(shapeAABB)
        const snap = snapMove(box, others, snapThreshold(vp().scale))
        dx += snap.x
        dy += snap.y
        ui.guides = snap.guides
      }
      for (const id of mode.ids) {
        const st = mode.starts[id]
        if (!st || !doc.shapes[id]) continue
        doc.shapes[id].x = st.x + dx
        doc.shapes[id].y = st.y + dy
      }
      doc.markStatic()
      return
    }
    if (mode.t === 'resize') {
      resizeLive(mode.start, mode.handle, p.wx, p.wy)
      doc.markStatic()
      return
    }
    if (mode.t === 'rotate') {
      const s = doc.shapes[mode.id]
      if (!s) return
      const cx = mode.start.x + mode.start.w / 2
      const cy = mode.start.y + mode.start.h / 2
      s.rotation = (Math.atan2(p.wy - cy, p.wx - cx) * 180) / Math.PI + 90
      doc.markStatic()
      return
    }
    if (mode.t === 'marquee') {
      ui.marquee = {
        minX: Math.min(mode.x0, p.wx),
        minY: Math.min(mode.y0, p.wy),
        maxX: Math.max(mode.x0, p.wx),
        maxY: Math.max(mode.y0, p.wy),
      }
      doc.markOverlay()
    }
  }

  function resizeLive(start: Shape, handle: HandleName, wx: number, wy: number) {
    const s = doc.shapes[start.id]
    if (!s) return
    if (handle === 'start' || handle === 'end') {
      const pts = start.points && start.points.length >= 2 ? start.points.map((p) => ({ ...p })) : [
        { x: 0, y: 0 },
        { x: start.w, y: start.h },
      ]
      if (handle === 'start') {
        const att = attachArrow(s.kind, wx, wy, s.id)
        pts[0] = { x: wx - start.x, y: wy - start.y }
        s.startId = att.id || ''
        s.startAnchor = att.anchor || ''
      } else {
        const att = attachArrow(s.kind, wx, wy, s.id)
        pts[pts.length - 1] = { x: wx - start.x, y: wy - start.y }
        s.endId = att.id || ''
        s.endAnchor = att.anchor || ''
      }
      s.points = pts
      return
    }
    const min = 8
    let { x, y, w, h } = start
    const right = x + w
    const bottom = y + h
    if (handle.includes('e')) w = Math.max(min, wx - x)
    if (handle.includes('s')) h = Math.max(min, wy - y)
    if (handle.includes('w')) {
      const nx = Math.min(wx, right - min)
      w = right - nx
      x = nx
    }
    if (handle.includes('n')) {
      const ny = Math.min(wy, bottom - min)
      h = bottom - ny
      y = ny
    }
    s.x = x
    s.y = y
    s.w = w
    s.h = h
  }

  function onPointerUp() {
    if (mode.t === 'draw' && ui.preview) {
      const s = ui.preview
      if ((s.kind === 'text' || s.kind === 'sticky') && s.w < 12 && s.h < 12) {
        s.w = s.kind === 'sticky' ? 180 : 220
        s.h = s.kind === 'sticky' ? 160 : 48
      }
      if (s.w >= 2 || s.h >= 2 || s.kind === 'line' || s.kind === 'arrow' || s.kind === 'text') {
        doc.createShape(s)
        ui.select([s.id])
        session.sendSelection([s.id])
        if (s.kind === 'text' || s.kind === 'sticky') ui.editingTextId = s.id
      }
      ui.preview = null
      ui.tool = 'select'
    } else if (mode.t === 'pen' && ui.preview) {
      if ((ui.preview.points?.length || 0) >= 2) {
        doc.createShape(ui.preview)
        ui.select([ui.preview.id])
      }
      ui.preview = null
      ui.tool = 'select'
    } else if (mode.t === 'move') {
      const patches = mode.ids.map((id) => {
        const s = doc.shapes[id]
        const st = mode.starts[id]
        if (!s || !st) return { x: 0, y: 0 }
        return { x: s.x, y: s.y }
      })
      // revert local then send through store so undo/pending is consistent
      for (const id of mode.ids) {
        const st = mode.starts[id]
        if (st && doc.shapes[id]) {
          doc.shapes[id].x = st.x
          doc.shapes[id].y = st.y
        }
      }
      doc.updateMany(mode.ids, patches)
      ui.guides = []
    } else if (mode.t === 'resize') {
      const s = doc.shapes[mode.id]
      const st = mode.start
      if (s) {
        const patch = {
          x: s.x,
          y: s.y,
          w: s.w,
          h: s.h,
          points: s.points,
          startId: s.startId,
          endId: s.endId,
          startAnchor: s.startAnchor,
          endAnchor: s.endAnchor,
        }
        s.x = st.x
        s.y = st.y
        s.w = st.w
        s.h = st.h
        s.points = st.points
        s.startId = st.startId
        s.endId = st.endId
        s.startAnchor = st.startAnchor
        s.endAnchor = st.endAnchor
        doc.updateShape(mode.id, patch)
      }
    } else if (mode.t === 'rotate') {
      const s = doc.shapes[mode.id]
      if (s) {
        const rot = s.rotation
        s.rotation = mode.start.rotation
        doc.updateShape(mode.id, { rotation: rot })
      }
    } else if (mode.t === 'marquee' && ui.marquee) {
      const hits = marqueeHits(doc.liveList, ui.marquee)
      ui.select(doc.expandGroup(hits))
      session.sendSelection(ui.selectedIds)
      ui.marquee = null
    }
    mode = { t: 'idle' }
    doc.markStatic()
  }

  function onWheel(e: WheelEvent) {
    e.preventDefault()
    const r = wrap.value?.getBoundingClientRect()
    if (!r) return
    const sx = e.clientX - r.left
    const sy = e.clientY - r.top
    const factor = e.deltaY > 0 ? 0.91 : 1.1
    ui.viewport = zoomAt(vp(), sx, sy, vp().scale * factor)
    doc.markStatic()
  }

  function onKeyDown(e: KeyboardEvent) {
    if (isTypingTarget(e.target) || ui.editingTextId) {
      if (e.key === 'Escape') ui.editingTextId = null
      return
    }
    if (e.code === 'Space') {
      ui.spaceDown = true
      e.preventDefault()
      return
    }
    const meta = e.metaKey || e.ctrlKey
    if (meta && e.key.toLowerCase() === 'z') {
      e.preventDefault()
      if (e.shiftKey) undoStack.doRedo()
      else undoStack.doUndo()
      return
    }
    if (meta && e.key.toLowerCase() === 'y') {
      e.preventDefault()
      undoStack.doRedo()
      return
    }
    if (meta && e.key.toLowerCase() === 'c') {
      e.preventDefault()
      doc.copy(ui.selectedIds)
      return
    }
    if (meta && e.key.toLowerCase() === 'v') {
      e.preventDefault()
      doc.paste()
      return
    }
    if (meta && e.key.toLowerCase() === 'd') {
      e.preventDefault()
      doc.duplicate(ui.selectedIds)
      return
    }
    if (e.key === 'Delete' || e.key === 'Backspace') {
      e.preventDefault()
      if (ui.selectedIds.length) doc.deleteShapes(ui.selectedIds)
      return
    }
    if (e.key === 'Escape') {
      ui.select([])
      ui.tool = 'select'
      ui.toolbarOpen = false
      return
    }
    const map: Record<string, Tool> = {
      v: 'select',
      r: 'rect',
      o: 'ellipse',
      d: 'diamond',
      l: 'line',
      a: 'arrow',
      p: 'freedraw',
      t: 'text',
      n: 'sticky',
    }
    const t = map[e.key.toLowerCase()]
    if (t && !meta) ui.tool = t
  }

  function onKeyUp(e: KeyboardEvent) {
    if (e.code === 'Space') ui.spaceDown = false
  }

  function onDblClick(e: MouseEvent) {
    const r = wrap.value?.getBoundingClientRect()
    if (!r) return
    const w = screenToWorld(e.clientX - r.left, e.clientY - r.top, vp())
    const hit = pickTop(doc.liveList, w.x, w.y, 4)
    if (hit && (hit.kind === 'text' || hit.kind === 'sticky')) {
      ui.select([hit.id])
      ui.editingTextId = hit.id
    }
  }

  function fitContent() {
    const r = wrap.value?.getBoundingClientRect()
    if (!r) return
    ui.fitTo(contentBounds(doc.liveList), r.width, r.height)
    doc.markStatic()
  }

  watch(
    () => [session.boardId, session.selfId, doc.lastServerSeq, ui.selectedIds.join(','), ui.tool, ui.preview, session.cursors, session.remoteSel],
    () => {
      doc.markOverlay()
    },
    { deep: true },
  )

  watch(
    () => [ui.viewport.x, ui.viewport.y, ui.viewport.scale, session.theme],
    () => doc.markStatic(),
  )

  onMounted(() => {
    raf = requestAnimationFrame(loop)
    wrap.value?.addEventListener('wheel', onWheel, { passive: false })
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    if (wrap.value) {
      ro = new ResizeObserver(() => doc.markStatic())
      ro.observe(wrap.value)
    }
  })
  onUnmounted(() => {
    cancelAnimationFrame(raf)
    wrap.value?.removeEventListener('wheel', onWheel)
    window.removeEventListener('keydown', onKeyDown)
    window.removeEventListener('keyup', onKeyUp)
    ro?.disconnect()
  })

  return { onPointerDown, onPointerMove, onPointerUp, onDblClick, fitContent, size }
}
