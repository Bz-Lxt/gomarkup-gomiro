const C2S = 0x01
const S2C = 0x02

export function encodeCursorC2S(x: number, y: number): ArrayBuffer {
  const buf = new ArrayBuffer(9)
  const v = new DataView(buf)
  v.setUint8(0, C2S)
  v.setFloat32(1, x, true)
  v.setFloat32(5, y, true)
  return buf
}

export type CursorSample = { userIdx: number; x: number; y: number }

export function decodeCursorS2C(data: ArrayBuffer): CursorSample[] | null {
  if (data.byteLength < 2) return null
  const v = new DataView(data)
  if (v.getUint8(0) !== S2C) return null
  const n = v.getUint8(1)
  const need = 2 + n * 12
  if (data.byteLength < need) return null
  const out: CursorSample[] = []
  let off = 2
  for (let i = 0; i < n; i++) {
    const userIdx = v.getUint32(off, true)
    const x = v.getFloat32(off + 4, true)
    const y = v.getFloat32(off + 8, true)
    if (!Number.isFinite(x) || !Number.isFinite(y)) return null
    out.push({ userIdx, x, y })
    off += 12
  }
  return out
}
