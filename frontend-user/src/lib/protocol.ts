export const PROTO_VERSION = 1

export const KINDS = [
  'rect',
  'ellipse',
  'diamond',
  'line',
  'arrow',
  'freedraw',
  'text',
  'sticky',
  'image',
] as const

export type ShapeKind = (typeof KINDS)[number]

export type Anchor = 'n' | 's' | 'w' | 'e' | 'nw' | 'ne' | 'sw' | 'se' | 'c'

export type Point = { x: number; y: number; p?: number }

export type Shape = {
  id: string
  kind: ShapeKind
  x: number
  y: number
  w: number
  h: number
  rotation: number
  stroke: string
  fill: string
  strokeW: number
  dash: 'solid' | 'dashed'
  opacity: number
  z: number
  text?: string
  fontSize?: number
  align?: 'left' | 'center' | 'right'
  points?: Point[]
  startId?: string
  endId?: string
  startAnchor?: string
  endAnchor?: string
  imageUrl?: string
  groupId?: string
  radius?: number
  deleted?: boolean
  version: number
  lastWriterId: string
  updatedAt: number
  propVer?: Record<string, number>
}

export type Member = {
  id: string
  nickname: string
  color: string
  userIdx: number
  cursorX?: number
  cursorY?: number
  online: boolean
}

export type Snapshot = {
  boardId: string
  serverSeq: number
  shapes: Record<string, Shape>
  groups?: Record<string, string[]>
}

export type OpKind =
  | 'shape.create'
  | 'shape.update'
  | 'shape.delete'
  | 'shape.reorder'
  | 'shapes.group'
  | 'shapes.ungroup'

export type Envelope<T = unknown> = {
  v: number
  type: string
  id?: string
  payload?: T
}

export type JoinPayload = {
  boardId: string
  nickname: string
  color: string
  passcode?: string
  lastSeq: number
  protoVersion: number
}

export type OpPayload = {
  clientOpId: string
  lamport: number
  baseVersion: number
  opKind: OpKind
  targetId: string
  patch?: unknown
}

export type OpBroadcast = {
  serverSeq: number
  authorId: string
  opKind: OpKind
  targetId: string
  patch?: unknown
  lamport: number
  version: number
}

export type JoinedPayload = {
  selfId: string
  userIdx: number
  members: Member[]
  snapshot: Snapshot
  serverSeq: number
  missed?: OpBroadcast[]
}

export type OpAckPayload = {
  clientOpId: string
  serverSeq: number
  acceptedVersion: number
}

export type OpRejectPayload = {
  clientOpId: string
  reason: string
  authoritativeShape?: Shape
}

export type BoardListItem = {
  id: string
  title: string
  hasPass: boolean
  thumbnail?: string
  createdAt: string
  updatedAt: string
}

export const PATCH_FIELDS = [
  'x',
  'y',
  'w',
  'h',
  'rotation',
  'stroke',
  'fill',
  'strokeW',
  'dash',
  'opacity',
  'z',
  'text',
  'fontSize',
  'align',
  'points',
  'startId',
  'endId',
  'startAnchor',
  'endAnchor',
  'imageUrl',
  'groupId',
  'radius',
] as const

export type Patch = Partial<
  Pick<
    Shape,
    | 'x'
    | 'y'
    | 'w'
    | 'h'
    | 'rotation'
    | 'stroke'
    | 'fill'
    | 'strokeW'
    | 'dash'
    | 'opacity'
    | 'z'
    | 'text'
    | 'fontSize'
    | 'align'
    | 'points'
    | 'startId'
    | 'endId'
    | 'startAnchor'
    | 'endAnchor'
    | 'imageUrl'
    | 'groupId'
    | 'radius'
  >
>

export function isFiniteNum(n: unknown): n is number {
  return typeof n === 'number' && Number.isFinite(n)
}

export function validColor(s: string): boolean {
  return /^#[0-9a-fA-F]{6}$/.test(s)
}

export function validFill(s: string): boolean {
  return validColor(s) || /^#[0-9a-fA-F]{8}$/.test(s)
}

export function cloneShape(s: Shape): Shape {
  return {
    ...s,
    points: s.points ? s.points.map((p) => ({ ...p })) : undefined,
    propVer: s.propVer ? { ...s.propVer } : undefined,
  }
}

export function applyPatch(s: Shape, patch: Patch): void {
  const rec = s as unknown as Record<string, unknown>
  for (const [k, v] of Object.entries(patch)) {
    if (v === undefined) continue
    if (k === 'points' && Array.isArray(v)) {
      s.points = (v as Point[]).map((p) => ({ ...p }))
      continue
    }
    rec[k] = v
  }
}

export function envelope<T>(type: string, payload?: T, id?: string): Envelope<T> {
  return { v: PROTO_VERSION, type, id, payload }
}
