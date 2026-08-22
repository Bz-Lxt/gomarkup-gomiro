const dev = import.meta.env.DEV

function fmt(level: string, args: unknown[]): unknown[] {
  return [`[gomiro:${level}]`, ...args]
}

export const logger = {
  debug(...args: unknown[]) {
    if (dev) console.debug(...fmt('debug', args))
  },
  info(...args: unknown[]) {
    if (dev) console.info(...fmt('info', args))
  },
  warn(...args: unknown[]) {
    console.warn(...fmt('warn', args))
  },
  error(...args: unknown[]) {
    console.error(...fmt('error', args))
  },
}
