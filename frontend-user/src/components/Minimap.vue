<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { drawMinimap, resizeCanvas } from '@/lib/render'
import { useDocStore } from '@/stores/doc'
import { useUiStore } from '@/stores/ui'

const props = defineProps<{ viewW: number; viewH: number }>()
const emit = defineEmits<{ fit: [] }>()

const doc = useDocStore()
const ui = useUiStore()
const canvas = ref<HTMLCanvasElement | null>(null)
let xform = { scale: 1, ox: 0, oy: 0 }

function paint() {
  const el = canvas.value
  if (!el) return
  const ctx = resizeCanvas(el, 168, 112)
  if (!ctx) return
  xform = drawMinimap(ctx, doc.liveList, ui.viewport, 168, 112, props.viewW, props.viewH)
}

function jump(e: MouseEvent) {
  const r = canvas.value?.getBoundingClientRect()
  if (!r) return
  const sx = e.clientX - r.left
  const sy = e.clientY - r.top
  const wx = (sx - xform.ox) / xform.scale
  const wy = (sy - xform.oy) / xform.scale
  ui.viewport = {
    ...ui.viewport,
    x: wx - props.viewW / ui.viewport.scale / 2,
    y: wy - props.viewH / ui.viewport.scale / 2,
  }
  doc.markStatic()
}

watch(() => [doc.dirtyStatic, ui.viewport.x, ui.viewport.y, ui.viewport.scale, props.viewW, props.viewH], paint)
onMounted(paint)
onUnmounted(() => undefined)
</script>

<template>
  <div class="glass-panel fixed bottom-4 right-4 z-30 hidden overflow-hidden rounded-2xl md:block">
    <canvas ref="canvas" class="block cursor-pointer" width="168" height="112" @click="jump" />
    <button type="button" class="w-full border-t border-brass/20 py-1.5 text-[11px] tracking-wide text-brass" @click="emit('fit')">
      回到内容
    </button>
  </div>
</template>
