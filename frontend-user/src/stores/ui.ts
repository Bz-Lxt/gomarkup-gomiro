import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { AABB, Guide, Viewport } from '@/lib/geometry'
import type { Shape, ShapeKind } from '@/lib/protocol'

export type Tool = 'select' | ShapeKind

export type ToastKind = 'info' | 'ok' | 'warn' | 'err'

export type Toast = {
  id: number
  message: string
  kind: ToastKind
}

export type ModalState = {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  input?: { label: string; value: string; placeholder?: string; password?: boolean }
  onConfirm: (input?: string) => void
}

let toastSeq = 1

export const useUiStore = defineStore('ui', () => {
  const tool = ref<Tool>('select')
  const selectedIds = ref<string[]>([])
  const viewport = ref<Viewport>({ x: -80, y: -60, scale: 1 })
  const toasts = ref<Toast[]>([])
  const modal = ref<ModalState | null>(null)
  const toolbarOpen = ref(false)
  const styleOpen = ref(false)
  const spaceDown = ref(false)
  const guides = ref<Guide[]>([])
  const marquee = ref<AABB | null>(null)
  const editingTextId = ref<string | null>(null)
  const preview = ref<Shape | null>(null)
  const nickOpen = ref(false)
  const passOpen = ref(false)

  const primaryId = computed(() => selectedIds.value[0] ?? null)

  function select(ids: string[], additive = false) {
    if (additive) {
      const set = new Set(selectedIds.value)
      for (const id of ids) {
        if (set.has(id)) set.delete(id)
        else set.add(id)
      }
      selectedIds.value = [...set]
    } else {
      selectedIds.value = [...ids]
    }
  }

  function toast(message: string, kind: ToastKind = 'info') {
    const id = toastSeq++
    toasts.value.push({ id, message, kind })
    window.setTimeout(() => dismiss(id), 5000)
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  function ask(state: ModalState) {
    modal.value = state
  }

  function confirmDanger(title: string, message: string): Promise<boolean> {
    return new Promise((resolve) => {
      modal.value = {
        title,
        message,
        danger: true,
        confirmLabel: '删除',
        cancelLabel: '取消',
        onConfirm: () => resolve(true),
      }
      const prev = modal.value
      modal.value = {
        ...prev,
        onConfirm: () => {
          modal.value = null
          resolve(true)
        },
      }
    })
  }

  function closeModal(ok = false, input?: string) {
    const m = modal.value
    modal.value = null
    if (ok) m?.onConfirm(input)
  }

  function fitTo(bounds: AABB, viewW: number, viewH: number) {
    const bw = Math.max(80, bounds.maxX - bounds.minX)
    const bh = Math.max(80, bounds.maxY - bounds.minY)
    const scale = Math.min(2, Math.max(0.15, Math.min((viewW - 80) / bw, (viewH - 80) / bh)))
    viewport.value = {
      scale,
      x: bounds.minX - (viewW / scale - bw) / 2,
      y: bounds.minY - (viewH / scale - bh) / 2,
    }
  }

  return {
    tool,
    selectedIds,
    primaryId,
    viewport,
    toasts,
    modal,
    toolbarOpen,
    styleOpen,
    spaceDown,
    guides,
    marquee,
    editingTextId,
    preview,
    nickOpen,
    passOpen,
    select,
    toast,
    dismiss,
    ask,
    confirmDanger,
    closeModal,
    fitTo,
  }
})
