const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000

function pad(n: number): string {
  return n < 10 ? `0${n}` : String(n)
}

export function formatBeijing(input?: string | number | Date | null): string {
  if (input == null || input === '') return ''
  if (typeof input === 'string' && /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(input)) {
    return input
  }
  const d = input instanceof Date ? input : new Date(input)
  if (Number.isNaN(d.getTime())) return String(input)
  const t = new Date(d.getTime() + BEIJING_OFFSET_MS)
  return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())} ${pad(t.getUTCHours())}:${pad(t.getUTCMinutes())}:${pad(t.getUTCSeconds())}`
}

export function nowBeijing(): string {
  return formatBeijing(new Date())
}
