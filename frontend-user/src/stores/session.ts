import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { BoardSocket, type ConnState } from '@/lib/socket'
import { logger } from '@/lib/logger'
import { PROTO_VERSION, type JoinedPayload, type Member, type OpAckPayload, type OpBroadcast, type OpPayload, type OpRejectPayload, type Snapshot } from '@/lib/protocol'
import { useDocStore } from './doc'
import { useUiStore } from './ui'

const NICK_KEY = 'gomiro.nickname'
const COLOR_KEY = 'gomiro.color'
const THEME_KEY = 'gomiro.theme'
const PASS_KEY = 'gomiro.pass.'

const PALETTE = ['#c45c26', '#2f5d56', '#9b2c2c', '#3d5a80', '#c4a35a', '#4a7c59', '#8b5a2b', '#9a3412']

function randomNick(): string {
  const n = crypto.getRandomValues(new Uint8Array(2))
  const hex = Array.from(n, (b) => b.toString(16).padStart(2, '0')).join('')
  return `制图员-${hex}`
}

function randomColor(): string {
  return PALETTE[Math.floor(Math.random() * PALETTE.length)]
}

function applyTheme(t: 'dark' | 'light') {
  document.documentElement.classList.toggle('dark', t === 'dark')
}

export const useSessionStore = defineStore('session', () => {
  const nickname = ref(localStorage.getItem(NICK_KEY) || '')
  const color = ref(localStorage.getItem(COLOR_KEY) || '')
  const theme = ref<'dark' | 'light'>((localStorage.getItem(THEME_KEY) as 'dark' | 'light') || 'dark')
  const status = ref<ConnState>('disconnected')
  const selfId = ref('')
  const userIdx = ref(0)
  const members = ref<Member[]>([])
  const cursors = ref<Record<number, { x: number; y: number }>>({})
  const remoteSel = ref<Record<string, string[]>>({})
  const boardId = ref('')
  const passcode = ref('')
  const boardTitle = ref('')

  let sock: BoardSocket | null = null

  if (!nickname.value) nickname.value = randomNick()
  if (!color.value) color.value = randomColor()
  localStorage.setItem(NICK_KEY, nickname.value)
  localStorage.setItem(COLOR_KEY, color.value)
  applyTheme(theme.value)

  const others = computed(() => members.value.filter((m) => m.id !== selfId.value && m.online))

  watch(nickname, (v) => localStorage.setItem(NICK_KEY, v))
  watch(color, (v) => localStorage.setItem(COLOR_KEY, v))
  watch(theme, (v) => {
    localStorage.setItem(THEME_KEY, v)
    applyTheme(v)
  })

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }

  function persistIdentity(nick: string, col: string) {
    nickname.value = nick.trim().slice(0, 24) || randomNick()
    color.value = col
  }

  function loadPass(id: string) {
    passcode.value = sessionStorage.getItem(PASS_KEY + id) || ''
  }

  function savePass(id: string, code: string) {
    passcode.value = code
    if (code) sessionStorage.setItem(PASS_KEY + id, code)
    else sessionStorage.removeItem(PASS_KEY + id)
  }

  function sendOp(clientOpId: string, payload: OpPayload) {
    sock?.sendOp(clientOpId, payload)
  }

  function sendSelection(ids: string[]) {
    sock?.sendSelection(ids)
  }

  function sendCursor(x: number, y: number) {
    sock?.sendCursor(x, y)
  }

  function resync() {
    const doc = useDocStore()
    sock?.updateJoin({ lastSeq: doc.lastServerSeq })
    sock?.disconnect()
    connect(boardId.value)
  }

  function connect(id: string) {
    const doc = useDocStore()
    const ui = useUiStore()
    boardId.value = id
    loadPass(id)
    sock?.disconnect()
    sock = new BoardSocket({
      onStatus: (s) => {
        status.value = s
      },
      onJoined: (raw) => {
        const p = raw as JoinedPayload
        selfId.value = p.selfId
        userIdx.value = p.userIdx
        members.value = p.members ?? []
        cursors.value = {}
        remoteSel.value = {}
        doc.applyJoined(p)
        sock?.updateJoin({ lastSeq: doc.lastServerSeq })
      },
      onAck: (raw) => doc.applyAck(raw as OpAckPayload),
      onReject: (raw) => doc.applyReject(raw as OpRejectPayload),
      onBcast: (raw) => doc.applyBcast(raw as OpBroadcast),
      onMemberJoin: (raw) => {
        const m = (raw as { member: Member }).member
        if (!m) return
        const i = members.value.findIndex((x) => x.id === m.id)
        if (i >= 0) members.value[i] = { ...m, online: true }
        else members.value.push({ ...m, online: true })
      },
      onMemberLeave: (raw) => {
        const m = (raw as { member: Member }).member
        if (!m) return
        members.value = members.value.filter((x) => x.id !== m.id)
        delete remoteSel.value[m.id]
        delete cursors.value[m.userIdx]
      },
      onSelection: (raw) => {
        const p = raw as { clientId: string; shapeIds: string[] }
        if (!p?.clientId || p.clientId === selfId.value) return
        remoteSel.value = { ...remoteSel.value, [p.clientId]: p.shapeIds ?? [] }
      },
      onCursors: (samples) => {
        const next = { ...cursors.value }
        for (const s of samples) {
          if (s.userIdx === userIdx.value) continue
          next[s.userIdx] = { x: s.x, y: s.y }
        }
        cursors.value = next
      },
      onResync: (raw) => {
        const p = raw as { snapshot?: Snapshot; serverSeq?: number; reason?: string }
        if (p?.snapshot) doc.applySnapshot(p.snapshot)
        ui.toast(p?.reason ? `需要重同步：${p.reason}` : '正在重同步画布', 'warn')
      },
      onError: (raw) => {
        const p = raw as { code?: string; message?: string }
        const msg = p?.message || '连接错误'
        logger.warn('ws error', p)
        if (msg.includes('passcode')) {
          ui.passOpen = true
          ui.toast('口令不正确', 'err')
        } else {
          ui.toast(msg, 'err')
        }
      },
      onShutdown: () => {
        ui.toast('服务即将重启，正在重连', 'warn')
        resync()
      },
    })
    sock.connect({
      boardId: id,
      nickname: nickname.value,
      color: color.value,
      passcode: passcode.value || undefined,
      lastSeq: doc.lastServerSeq,
      protoVersion: PROTO_VERSION,
    })
  }

  function disconnect() {
    sock?.disconnect()
    sock = null
    selfId.value = ''
    members.value = []
    cursors.value = {}
    remoteSel.value = {}
  }

  return {
    nickname,
    color,
    theme,
    status,
    selfId,
    userIdx,
    members,
    others,
    cursors,
    remoteSel,
    boardId,
    passcode,
    boardTitle,
    persistIdentity,
    toggleTheme,
    loadPass,
    savePass,
    sendOp,
    sendSelection,
    sendCursor,
    connect,
    disconnect,
    resync,
  }
})
