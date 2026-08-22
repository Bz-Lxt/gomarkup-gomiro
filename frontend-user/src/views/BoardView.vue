<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, reportApiError } from '@/lib/api'
import { downloadBlob, downloadText, exportJSON, exportPNG, exportSVG, thumbnailDataURL } from '@/lib/export'
import { worldToScreen } from '@/lib/geometry'
import { useCanvas } from '@/composables/useCanvas'
import { makeShape, useDocStore } from '@/stores/doc'
import { useSessionStore } from '@/stores/session'
import { useUiStore } from '@/stores/ui'
import Toolbar from '@/components/Toolbar.vue'
import StylePanel from '@/components/StylePanel.vue'
import MemberRail from '@/components/MemberRail.vue'
import Minimap from '@/components/Minimap.vue'
import ConnBadge from '@/components/ConnBadge.vue'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const doc = useDocStore()
const ui = useUiStore()

const wrap = ref<HTMLElement | null>(null)
const staticEl = ref<HTMLCanvasElement | null>(null)
const overlayEl = ref<HTMLCanvasElement | null>(null)
const fileEl = ref<HTMLInputElement | null>(null)
const editText = ref('')
const { onPointerDown, onPointerMove, onPointerUp, onDblClick, fitContent, size } = useCanvas(wrap, staticEl, overlayEl)

const boardId = computed(() => String(route.params.id || ''))
const editing = computed(() => (ui.editingTextId ? doc.shapes[ui.editingTextId] : null))
const editStyle = computed(() => {
  const s = editing.value
  if (!s || !wrap.value) return {}
  const r = wrap.value.getBoundingClientRect()
  const a = worldToScreen(s.x, s.y, ui.viewport)
  return {
    left: `${a.x}px`,
    top: `${a.y}px`,
    width: `${Math.max(80, s.w * ui.viewport.scale)}px`,
    height: `${Math.max(40, s.h * ui.viewport.scale)}px`,
    fontSize: `${(s.fontSize || 16) * ui.viewport.scale}px`,
    color: s.stroke,
  }
})

watch(
  () => ui.editingTextId,
  (id) => {
    if (id && doc.shapes[id]) editText.value = doc.shapes[id].text || ''
  },
)

function commitText() {
  const id = ui.editingTextId
  if (id && doc.shapes[id]) doc.updateShape(id, { text: editText.value })
  ui.editingTextId = null
}

async function bootstrap() {
  const id = boardId.value
  if (!id) return
  doc.clearDoc()
  session.boardTitle = ''
  try {
    const b = await api.getBoard(id)
    session.boardTitle = b.title
    session.loadPass(id)
    if (b.hasPass && !session.passcode) {
      ui.passOpen = true
      return
    }
    session.connect(id)
  } catch (e) {
    ui.toast(reportApiError(e, '白板不存在'), 'err')
    router.replace('/')
  }
}

watch(
  boardId,
  (id) => {
    if (id) bootstrap()
  },
  { immediate: true },
)

watch(
  () => [session.boardId, session.selfId, ui.primaryId, ui.selectedIds.join(',')],
  () => {
    doc.markOverlay()
  },
)

let thumbTimer = 0
watch(
  () => doc.lastServerSeq,
  () => {
    window.clearTimeout(thumbTimer)
    thumbTimer = window.setTimeout(() => {
      const id = session.boardId
      if (!id || !doc.liveList.length) return
      const thumb = thumbnailDataURL(doc.shapes)
      if (thumb) api.patchBoard(id, { thumbnail: thumb }).catch(() => undefined)
    }, 4000)
  },
)

async function onImage(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  try {
    const up = await api.upload(file)
    const img = new Image()
    img.src = up.url
    await img.decode().catch(() => undefined)
    const max = 480
    let w = img.naturalWidth || 320
    let h = img.naturalHeight || 240
    const s = Math.min(1, max / Math.max(w, h))
    w *= s
    h *= s
    const cx = ui.viewport.x + size.value.w / ui.viewport.scale / 2
    const cy = ui.viewport.y + size.value.h / ui.viewport.scale / 2
    const shape = makeShape('image', session.theme === 'dark', {
      x: cx - w / 2,
      y: cy - h / 2,
      w,
      h,
      imageUrl: up.url,
    })
    doc.createShape(shape)
    ui.select([shape.id])
    ui.toast('图片已置入', 'ok')
  } catch (err) {
    ui.toast(reportApiError(err, '上传失败（需 PNG/JPEG/WebP/GIF，≤5MB）'), 'err')
  } finally {
    if (fileEl.value) fileEl.value.value = ''
  }
}

async function doExport(kind: 'png' | 'svg' | 'json') {
  const name = session.boardTitle || 'board'
  try {
    if (kind === 'png') {
      const blob = await exportPNG(doc.shapes)
      downloadBlob(blob, `${name}.png`)
    } else if (kind === 'svg') {
      downloadText(exportSVG(doc.shapes), `${name}.svg`, 'image/svg+xml')
    } else {
      downloadText(exportJSON(session.boardId, doc.shapes, doc.groups), `${name}.json`, 'application/json')
    }
    ui.toast(`已导出 ${kind.toUpperCase()}`, 'ok')
  } catch (e) {
    ui.toast(reportApiError(e, '导出失败'), 'err')
  }
}

async function share() {
  try {
    await navigator.clipboard.writeText(location.href)
    ui.toast('当前链接已复制', 'ok')
  } catch {
    ui.toast(location.href, 'info')
  }
}

function rename() {
  ui.ask({
    title: '重命名白板',
    message: '写入图纸标题（最多 80 字）。',
    confirmLabel: '保存',
    input: { label: '标题 *', value: session.boardTitle },
    onConfirm: async (v) => {
      const n = (v || '').trim()
      if (!n || n.length > 80) {
        ui.toast('标题不合法', 'err')
        return
      }
      try {
        const b = await api.patchBoard(session.boardId, { title: n })
        session.boardTitle = b.title
        ui.toast('已更名', 'ok')
      } catch (e) {
        ui.toast(reportApiError(e, '更名失败'), 'err')
      }
    },
  })
}

onUnmounted(() => {
  window.clearTimeout(thumbTimer)
  session.disconnect()
  doc.clearDoc()
})
</script>

<template>
  <div class="relative h-screen w-full overflow-hidden bg-paper text-ink dark:bg-dusk dark:text-paper">
    <div
      ref="wrap"
      class="absolute inset-0 touch-none"
      :class="ui.spaceDown ? 'cursor-grab' : ui.tool === 'select' ? 'cursor-default' : 'cursor-crosshair'"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @dblclick="onDblClick"
    >
      <canvas ref="staticEl" class="absolute inset-0 h-full w-full" />
      <canvas ref="overlayEl" class="absolute inset-0 h-full w-full" />
      <textarea
        v-if="editing"
        v-model="editText"
        class="absolute z-20 resize-none border border-brass bg-paper/90 p-2 font-sans outline-none dark:bg-panel/90"
        :style="editStyle"
        @blur="commitText"
        @keydown.esc.prevent="commitText"
      />
    </div>

    <header class="pointer-events-none absolute inset-x-0 top-0 z-30 flex w-full items-start justify-between gap-3 p-3 md:p-4">
      <div class="pointer-events-auto flex flex-wrap items-center gap-2 pl-14 md:pl-20">
        <RouterLink to="/" class="font-display text-sm tracking-wide text-brass">← 目录</RouterLink>
        <button type="button" class="font-display text-base" @click="rename">{{ session.boardTitle || '未命名白板' }}</button>
        <ConnBadge />
      </div>
      <div class="pointer-events-auto flex flex-wrap justify-end gap-2">
        <button type="button" class="glass-panel rounded-full px-3 py-1.5 text-xs" @click="share">分享</button>
        <button type="button" class="glass-panel rounded-full px-3 py-1.5 text-xs" @click="doExport('png')">PNG</button>
        <button type="button" class="glass-panel rounded-full px-3 py-1.5 text-xs" @click="doExport('svg')">SVG</button>
        <button type="button" class="glass-panel rounded-full px-3 py-1.5 text-xs" @click="doExport('json')">JSON</button>
        <button type="button" class="glass-panel rounded-full px-3 py-1.5 text-xs" @click="session.toggleTheme()">
          {{ session.theme === 'dark' ? '浅色' : '深色' }}
        </button>
        <button type="button" class="glass-panel rounded-full px-3 py-1.5 text-xs md:hidden" @click="ui.styleOpen = true">属性</button>
        <button type="button" class="glass-panel rounded-full px-3 py-1.5 text-xs md:hidden" @click="fitContent">归位</button>
      </div>
    </header>

    <Toolbar @image="fileEl?.click()" />
    <StylePanel />
    <MemberRail />
    <Minimap :view-w="size.w" :view-h="size.h" @fit="fitContent" />
    <input ref="fileEl" type="file" accept="image/png,image/jpeg,image/webp,image/gif" class="hidden" @change="onImage" />
  </div>
</template>
