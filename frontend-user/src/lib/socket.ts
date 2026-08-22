import { decodeCursorS2C, encodeCursorC2S } from './binary'
import { logger } from './logger'
import { envelope, type Envelope, type JoinPayload, type OpPayload } from './protocol'

export type ConnState = 'connected' | 'reconnecting' | 'disconnected'

export type SocketHandlers = {
  onStatus: (s: ConnState) => void
  onJoined: (payload: unknown) => void
  onAck: (payload: unknown) => void
  onReject: (payload: unknown) => void
  onBcast: (payload: unknown) => void
  onMemberJoin: (payload: unknown) => void
  onMemberLeave: (payload: unknown) => void
  onSelection: (payload: unknown) => void
  onCursors: (samples: { userIdx: number; x: number; y: number }[]) => void
  onResync: (payload: unknown) => void
  onError: (payload: unknown) => void
  onShutdown: () => void
}

function wsUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}/ws`
}

export class BoardSocket {
  private ws: WebSocket | null = null
  private join: JoinPayload | null = null
  private attempt = 0
  private closedByUser = false
  private pingTimer = 0
  private reconnectTimer = 0
  private cursorAt = 0

  constructor(private handlers: SocketHandlers) {}

  connect(join: JoinPayload) {
    this.join = join
    this.closedByUser = false
    this.open()
  }

  updateJoin(partial: Partial<JoinPayload>) {
    if (this.join) this.join = { ...this.join, ...partial }
  }

  disconnect() {
    this.closedByUser = true
    window.clearTimeout(this.reconnectTimer)
    window.clearInterval(this.pingTimer)
    this.ws?.close()
    this.ws = null
    this.handlers.onStatus('disconnected')
  }

  private open() {
    if (this.closedByUser || !this.join) return
    this.handlers.onStatus(this.attempt === 0 ? 'reconnecting' : 'reconnecting')
    const ws = new WebSocket(wsUrl())
    ws.binaryType = 'arraybuffer'
    this.ws = ws
    ws.onopen = () => {
      ws.send(JSON.stringify(envelope('join', this.join)))
      this.attempt = 0
      this.handlers.onStatus('connected')
      window.clearInterval(this.pingTimer)
      this.pingTimer = window.setInterval(() => this.sendPing(), 25000)
      logger.debug('ws open, join sent')
    }
    ws.onmessage = (ev) => this.onMessage(ev)
    ws.onerror = () => logger.warn('ws error')
    ws.onclose = () => {
      window.clearInterval(this.pingTimer)
      this.ws = null
      if (this.closedByUser) return
      this.handlers.onStatus('reconnecting')
      const delay = Math.min(30000, 800 * 2 ** this.attempt)
      this.attempt += 1
      this.reconnectTimer = window.setTimeout(() => this.open(), delay)
      logger.info('ws closed, retry in', delay)
    }
  }

  private onMessage(ev: MessageEvent) {
    if (ev.data instanceof ArrayBuffer) {
      const samples = decodeCursorS2C(ev.data)
      if (samples) this.handlers.onCursors(samples)
      return
    }
    let env: Envelope
    try {
      env = JSON.parse(String(ev.data)) as Envelope
    } catch {
      logger.warn('bad json frame')
      return
    }
    switch (env.type) {
      case 'joined':
        this.handlers.onJoined(env.payload)
        break
      case 'op_ack':
        this.handlers.onAck(env.payload)
        break
      case 'op_reject':
        this.handlers.onReject(env.payload)
        break
      case 'op_bcast':
        this.handlers.onBcast(env.payload)
        break
      case 'member_join':
        this.handlers.onMemberJoin(env.payload)
        break
      case 'member_leave':
        this.handlers.onMemberLeave(env.payload)
        break
      case 'selection_bcast':
        this.handlers.onSelection(env.payload)
        break
      case 'resync_required':
        this.handlers.onResync(env.payload)
        break
      case 'error':
        this.handlers.onError(env.payload)
        break
      case 'server_shutdown':
        this.handlers.onShutdown()
        break
      case 'pong':
        break
      default:
        logger.debug('unhandled', env.type)
    }
  }

  sendOp(id: string, payload: OpPayload) {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify(envelope('op', payload, id)))
  }

  sendSelection(shapeIds: string[]) {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify(envelope('selection', { shapeIds })))
  }

  sendCursor(x: number, y: number) {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    const now = performance.now()
    if (now - this.cursorAt < 1000 / 30) return
    this.cursorAt = now
    if (!Number.isFinite(x) || !Number.isFinite(y)) return
    this.ws.send(encodeCursorC2S(x, y))
  }

  sendPing() {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify(envelope('ping')))
  }
}
