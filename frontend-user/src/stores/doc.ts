import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import { applyPatch, cloneShape, type JoinedPayload, type OpAckPayload, type OpBroadcast, type OpKind, type OpPayload, type OpRejectPayload, type Patch, type Shape, type ShapeKind, type Snapshot } from '@/lib/protocol'
import { newGroupId, newOpId, newShapeId } from '@/lib/ids'
import { nextZ, sortedShapes } from '@/lib/geometry'
import { undoStack } from '@/lib/undo'
import { logger } from '@/lib/logger'
import { useSessionStore } from './session'
import { useUiStore } from './ui'

type Pending = {
  clientOpId: string
  before: Record<string, Shape | null>
  groups?: Record<string, string[]>
}

export function defaultStyle(kind: ShapeKind, dark: boolean): Pick<Shape, 'stroke' | 'fill' | 'strokeW' | 'dash' | 'opacity' | 'fontSize' | 'align' | 'radius'> {
  const stroke = dark ? '#f3eadc' : '#1c1915'
  let fill = dark ? '#211c16' : '#f3eadc'
  if (kind === 'text' || kind === 'line' || kind === 'arrow' || kind === 'freedraw') fill = '#00000000'
  if (kind === 'sticky') fill = dark ? '#c4a35acc' : '#e4c56a'
  return {
    stroke,
    fill,
    strokeW: kind === 'freedraw' ? 3 : 2,
    dash: 'solid',
    opacity: 1,
    fontSize: kind === 'sticky' ? 16 : 18,
    align: kind === 'sticky' || kind === 'text' ? 'left' : 'center',
    radius: kind === 'rect' ? 4 : 0,
  }
}

export function makeShape(kind: ShapeKind, dark: boolean, extra: Partial<Shape> = {}): Shape {
  const style = defaultStyle(kind, dark)
  return {
    id: extra.id || newShapeId(),
    kind,
    x: extra.x ?? 0,
    y: extra.y ?? 0,
    w: extra.w ?? (kind === 'text' ? 220 : kind === 'sticky' ? 180 : 160),
    h: extra.h ?? (kind === 'text' ? 48 : kind === 'sticky' ? 160 : 96),
    rotation: extra.rotation ?? 0,
    ...style,
    ...extra,
    stroke: extra.stroke ?? style.stroke,
    fill: extra.fill ?? style.fill,
    z: extra.z ?? 0,
    version: extra.version ?? 0,
    lastWriterId: extra.lastWriterId ?? '',
    updatedAt: extra.updatedAt ?? 0,
    deleted: false,
  }
}

export const useDocStore = defineStore('doc', () => {
  const shapes = reactive<Record<string, Shape>>({})
  const groups = reactive<Record<string, string[]>>({})
  const lastServerSeq = ref(0)
  const lamport = ref(1)
  const clipboard = ref<Shape[]>([])
  const pending = new Map<string, Pending>()
  const dirtyStatic = ref(true)
  const dirtyOverlay = ref(true)

  const liveList = computed(() => sortedShapes(Object.values(shapes)))

  function markStatic() {
    dirtyStatic.value = true
    dirtyOverlay.value = true
  }
  function markOverlay() {
    dirtyOverlay.value = true
  }

  function clearDoc() {
    for (const k of Object.keys(shapes)) delete shapes[k]
    for (const k of Object.keys(groups)) delete groups[k]
    pending.clear()
    lastServerSeq.value = 0
    undoStack.clear()
    markStatic()
  }

  function put(s: Shape) {
    shapes[s.id] = s
    markStatic()
  }

  function applySnapshot(snap: Snapshot) {
    for (const k of Object.keys(shapes)) delete shapes[k]
    for (const k of Object.keys(groups)) delete groups[k]
    for (const [id, s] of Object.entries(snap.shapes || {})) {
      if (s) shapes[id] = cloneShape(s)
    }
    for (const [id, ids] of Object.entries(snap.groups || {})) groups[id] = [...ids]
    lastServerSeq.value = snap.serverSeq
    markStatic()
  }

  function applyJoined(p: JoinedPayload) {
    if (p.snapshot) applySnapshot(p.snapshot)
    lastServerSeq.value = p.snapshot?.serverSeq ?? p.serverSeq ?? 0
    for (const m of p.missed ?? []) {
      if (m.serverSeq > lastServerSeq.value) applyBcast(m)
    }
    if (p.serverSeq > lastServerSeq.value) lastServerSeq.value = p.serverSeq
  }

  function noteSeq(seq: number): boolean {
    if (!seq) return true
    if (seq <= lastServerSeq.value) return true
    if (seq === lastServerSeq.value + 1) {
      lastServerSeq.value = seq
      return true
    }
    logger.warn('seq hole', lastServerSeq.value, seq)
    useUiStore().toast('同步序号出现空洞，正在重拉快照', 'warn')
    useSessionStore().resync()
    return false
  }

  function applyFields(id: string, patch: Patch) {
    const s = shapes[id]
    if (!s || s.deleted) return
    applyPatch(s, patch)
    markStatic()
  }

  function applyRemote(b: OpBroadcast) {
    switch (b.opKind) {
      case 'shape.create': {
        const shape = (b.patch as { shape?: Shape })?.shape
        if (shape) put(cloneShape(shape))
        break
      }
      case 'shape.update':
      case 'shape.reorder': {
        if (shapes[b.targetId]) {
          applyFields(b.targetId, (b.patch || {}) as Patch)
          shapes[b.targetId].version = b.version || shapes[b.targetId].version
          shapes[b.targetId].lastWriterId = b.authorId
        }
        break
      }
      case 'shape.delete': {
        const ids = (b.patch as { ids?: string[] })?.ids ?? (b.targetId ? [b.targetId] : [])
        for (const id of ids) {
          if (shapes[id]) shapes[id].deleted = true
        }
        markStatic()
        break
      }
      case 'shapes.group': {
        const p = b.patch as { groupId: string; ids: string[] }
        if (!p?.groupId) break
        groups[p.groupId] = [...(p.ids || [])]
        for (const id of p.ids || []) {
          if (shapes[id]) shapes[id].groupId = p.groupId
        }
        markStatic()
        break
      }
      case 'shapes.ungroup': {
        const p = b.patch as { groupId: string; ids?: string[] }
        const ids = p?.ids || groups[p.groupId] || []
        for (const id of ids) {
          if (shapes[id]) shapes[id].groupId = ''
        }
        delete groups[p.groupId]
        markStatic()
        break
      }
    }
  }

  function applyBcast(b: OpBroadcast) {
    const session = useSessionStore()
    if (b.authorId === session.selfId) {
      noteSeq(b.serverSeq)
      return
    }
    applyRemote(b)
    noteSeq(b.serverSeq)
  }

  function applyAck(p: OpAckPayload) {
    const pend = pending.get(p.clientOpId)
    if (pend) {
      for (const id of Object.keys(pend.before)) {
        if (shapes[id] && !shapes[id].deleted) {
          shapes[id].version = p.acceptedVersion || shapes[id].version
        }
      }
      pending.delete(p.clientOpId)
    }
    noteSeq(p.serverSeq)
  }

  function applyReject(p: OpRejectPayload) {
    const pend = pending.get(p.clientOpId)
    if (pend) {
      for (const [id, prev] of Object.entries(pend.before)) {
        if (prev == null) delete shapes[id]
        else shapes[id] = cloneShape(prev)
      }
      if (pend.groups) {
        for (const k of Object.keys(groups)) delete groups[k]
        for (const [k, v] of Object.entries(pend.groups)) groups[k] = [...v]
      }
      pending.delete(p.clientOpId)
      markStatic()
    }
    if (p.authoritativeShape) put(cloneShape(p.authoritativeShape))
    useUiStore().toast(`操作被拒绝：${p.reason || 'conflict'}`, 'warn')
  }

  function tickLamport(): number {
    lamport.value += 1
    return lamport.value
  }

  function submit(opKind: OpKind, targetId: string, patch: unknown, before: Record<string, Shape | null>, groupSnap?: Record<string, string[]>, baseVersion = 0) {
    const session = useSessionStore()
    const clientOpId = newOpId()
    pending.set(clientOpId, { clientOpId, before, groups: groupSnap })
    const payload: OpPayload = {
      clientOpId,
      lamport: tickLamport(),
      baseVersion,
      opKind,
      targetId,
      patch,
    }
    session.sendOp(clientOpId, payload)
    return clientOpId
  }

  function createShape(shape: Shape, recordUndo = true) {
    if (!shape.z) shape.z = nextZ(Object.values(shapes))
    const before: Record<string, Shape | null> = { [shape.id]: null }
    put(cloneShape(shape))
    const id = submit('shape.create', shape.id, { shape: cloneShape(shape) }, before)
    if (recordUndo) {
      undoStack.push(
        () => deleteShapes([shape.id], false),
        () => {
          if (!shapes[shape.id] || shapes[shape.id].deleted) createShape(cloneShape(shape), false)
        },
      )
    }
    return id
  }

  function updateShape(id: string, patch: Patch, recordUndo = true) {
    const s = shapes[id]
    if (!s || s.deleted) return
    const before = { [id]: cloneShape(s) }
    const prev: Patch = {}
    for (const k of Object.keys(patch) as (keyof Patch)[]) {
      ;(prev as Record<string, unknown>)[k] = s[k as keyof Shape]
    }
    applyFields(id, patch)
    submit('shape.update', id, patch, before, undefined, s.version)
    if (recordUndo) {
      undoStack.push(
        () => updateShape(id, prev, false),
        () => updateShape(id, patch, false),
      )
    }
  }

  function updateMany(ids: string[], patches: Patch[], recordUndo = true) {
    if (!ids.length) return
    const befores: Record<string, Shape | null> = {}
    const prevs: { id: string; patch: Patch }[] = []
    ids.forEach((id, i) => {
      const s = shapes[id]
      if (!s || s.deleted) return
      befores[id] = cloneShape(s)
      const patch = patches[i]
      const prev: Patch = {}
      for (const k of Object.keys(patch) as (keyof Patch)[]) {
        ;(prev as Record<string, unknown>)[k] = s[k as keyof Shape]
      }
      prevs.push({ id, patch: prev })
      applyFields(id, patch)
      submit('shape.update', id, patch, { [id]: befores[id] }, undefined, s.version)
    })
    if (recordUndo && prevs.length) {
      undoStack.push(
        () => {
          for (const p of prevs) updateShape(p.id, p.patch, false)
        },
        () => {
          ids.forEach((id, i) => updateShape(id, patches[i], false))
        },
      )
    }
  }

  function deleteShapes(ids: string[], recordUndo = true) {
    const live = ids.filter((id) => shapes[id] && !shapes[id].deleted)
    if (!live.length) return
    const before: Record<string, Shape | null> = {}
    const restored: Shape[] = []
    for (const id of live) {
      before[id] = cloneShape(shapes[id])
      restored.push(cloneShape(shapes[id]))
      shapes[id].deleted = true
    }
    markStatic()
    submit('shape.delete', live[0], { ids: live }, before)
    if (recordUndo) {
      undoStack.push(
        () => {
          for (const s of restored) createShape(s, false)
        },
        () => deleteShapes(live, false),
      )
    }
    useUiStore().select([])
  }

  function reorder(id: string, place: 'top' | 'bottom' | 'up' | 'down') {
    const s = shapes[id]
    if (!s) return
    const before = { [id]: cloneShape(s) }
    submit('shape.reorder', id, { id, place }, before, undefined, s.version)
  }

  function groupSelected(ids: string[]) {
    const live = ids.filter((id) => shapes[id] && !shapes[id].deleted)
    if (live.length < 2) return
    const gid = newGroupId()
    const gsnap = Object.fromEntries(Object.entries(groups).map(([k, v]) => [k, [...v]]))
    const before: Record<string, Shape | null> = {}
    for (const id of live) {
      before[id] = cloneShape(shapes[id])
      shapes[id].groupId = gid
    }
    groups[gid] = [...live]
    markStatic()
    submit('shapes.group', gid, { groupId: gid, ids: live }, before, gsnap)
    undoStack.push(
      () => ungroup(gid, false),
      () => {
        const still = live.filter((id) => shapes[id] && !shapes[id].deleted)
        if (still.length >= 2) groupSelected(still)
      },
    )
  }

  function ungroup(groupId: string, recordUndo = true) {
    const ids = groups[groupId]
    if (!ids) return
    const gsnap = Object.fromEntries(Object.entries(groups).map(([k, v]) => [k, [...v]]))
    const before: Record<string, Shape | null> = {}
    for (const id of ids) {
      if (shapes[id]) {
        before[id] = cloneShape(shapes[id])
        shapes[id].groupId = ''
      }
    }
    delete groups[groupId]
    markStatic()
    submit('shapes.ungroup', groupId, { groupId, ids }, before, gsnap)
    if (recordUndo) {
      undoStack.push(
        () => groupSelected(ids),
        () => ungroup(groupId, false),
      )
    }
  }

  function expandGroup(ids: string[]): string[] {
    const set = new Set(ids)
    for (const id of ids) {
      const gid = shapes[id]?.groupId
      if (gid && groups[gid]) for (const x of groups[gid]) set.add(x)
    }
    return [...set]
  }

  function copy(ids: string[]) {
    clipboard.value = ids.map((id) => shapes[id]).filter(Boolean).filter((s) => !s.deleted).map(cloneShape)
    if (clipboard.value.length) useUiStore().toast(`已复制 ${clipboard.value.length} 个图元`, 'ok')
  }

  function paste(offset = 24) {
    const dark = useSessionStore().theme === 'dark'
    const created: string[] = []
    for (const src of clipboard.value) {
      const s = makeShape(src.kind, dark, {
        ...cloneShape(src),
        id: newShapeId(),
        x: src.x + offset,
        y: src.y + offset,
        groupId: '',
        version: 0,
      })
      createShape(s)
      created.push(s.id)
    }
    if (created.length) useUiStore().select(created)
  }

  function duplicate(ids: string[]) {
    copy(ids)
    paste(16)
  }

  return {
    shapes,
    groups,
    lastServerSeq,
    lamport,
    clipboard,
    liveList,
    dirtyStatic,
    dirtyOverlay,
    markStatic,
    markOverlay,
    clearDoc,
    applySnapshot,
    applyJoined,
    applyBcast,
    applyAck,
    applyReject,
    createShape,
    updateShape,
    updateMany,
    deleteShapes,
    reorder,
    groupSelected,
    ungroup,
    expandGroup,
    copy,
    paste,
    duplicate,
  }
})
