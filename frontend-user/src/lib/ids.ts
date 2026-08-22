const ID_RE = /^[A-Za-z0-9_-]{6,64}$/

export function validId(s: string): boolean {
  return ID_RE.test(s.trim())
}

export function newId(prefix: string): string {
  const bytes = new Uint8Array(10)
  crypto.getRandomValues(bytes)
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  return prefix ? `${prefix}_${hex}` : hex
}

export const newShapeId = () => newId('shp')
export const newOpId = () => newId('op')
export const newGroupId = () => newId('g')
