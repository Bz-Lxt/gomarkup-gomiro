<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDocStore } from '@/stores/doc'
import { useUiStore } from '@/stores/ui'

const doc = useDocStore()
const ui = useUiStore()

const stroke = ref('#f3eadc')
const fill = ref('#211c16')
const noFill = ref(false)
const strokeW = ref(2)
const dash = ref<'solid' | 'dashed'>('solid')
const opacity = ref(1)
const fontSize = ref(16)
const align = ref<'left' | 'center' | 'right'>('left')
const radius = ref(0)

const primary = computed(() => (ui.primaryId ? doc.shapes[ui.primaryId] : null))
const hasSel = computed(() => ui.selectedIds.length > 0)
const textLike = computed(() => primary.value && (primary.value.kind === 'text' || primary.value.kind === 'sticky'))

watch(
  () => [ui.primaryId, primary.value?.version, primary.value?.stroke, primary.value?.fill],
  () => {
    const s = primary.value
    if (!s) return
    stroke.value = s.stroke
    fill.value = s.fill.length === 9 ? s.fill.slice(0, 7) : s.fill === '#00000000' ? '#211c16' : s.fill
    noFill.value = s.fill === '#00000000'
    strokeW.value = s.strokeW
    dash.value = s.dash
    opacity.value = s.opacity
    fontSize.value = s.fontSize || 16
    align.value = s.align || 'left'
    radius.value = s.radius || 0
  },
)

function applyAll(patch: Record<string, unknown>) {
  if (!hasSel.value) return
  doc.updateMany(
    ui.selectedIds,
    ui.selectedIds.map(() => patch as never),
  )
}

function onStroke(v: string) {
  stroke.value = v
  applyAll({ stroke: v })
}
function onFill(v: string) {
  fill.value = v
  noFill.value = false
  applyAll({ fill: v })
}
function toggleFill() {
  noFill.value = !noFill.value
  applyAll({ fill: noFill.value ? '#00000000' : fill.value })
}
</script>

<template>
  <aside
    class="glass-panel z-30 flex-col gap-3 p-4"
    :class="
      ui.styleOpen
        ? 'flex fixed inset-x-0 bottom-0 max-h-[62vh] overflow-y-auto rounded-t-2xl md:hidden'
        : 'hidden md:pointer-events-auto md:fixed md:right-4 md:top-20 md:flex md:w-64 md:rounded-2xl'
    "
  >
    <div class="flex items-center justify-between">
      <p class="font-display text-sm tracking-wide">属性</p>
      <button type="button" class="text-xs opacity-60 md:hidden" @click="ui.styleOpen = false">收起</button>
    </div>
    <p v-if="!hasSel" class="text-xs opacity-60">选中图元后可改样式、层级与成组。</p>
    <template v-else>
      <label class="text-[11px] uppercase tracking-[0.14em] text-brass">描边
        <input :value="stroke" type="color" class="mt-1 h-9 w-full cursor-pointer rounded border border-brass/30 bg-transparent" @input="onStroke(($event.target as HTMLInputElement).value)" />
      </label>
      <label class="text-[11px] uppercase tracking-[0.14em] text-brass">填充
        <input :value="fill" type="color" class="mt-1 h-9 w-full cursor-pointer rounded border border-brass/30 bg-transparent" @input="onFill(($event.target as HTMLInputElement).value)" />
      </label>
      <button type="button" class="text-left text-xs text-tealink underline" @click="toggleFill">
        {{ noFill ? '恢复填充' : '设为无填充' }}
      </button>
      <label class="text-[11px] uppercase tracking-[0.14em] text-brass">线宽 {{ strokeW }}
        <input v-model.number="strokeW" type="range" min="0.5" max="16" step="0.5" class="mt-1 w-full" @change="applyAll({ strokeW })" />
      </label>
      <label class="text-[11px] uppercase tracking-[0.14em] text-brass">线型
        <select v-model="dash" class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm" @change="applyAll({ dash })">
          <option value="solid">实线</option>
          <option value="dashed">虚线</option>
        </select>
      </label>
      <label class="text-[11px] uppercase tracking-[0.14em] text-brass">透明度 {{ opacity.toFixed(2) }}
        <input v-model.number="opacity" type="range" min="0.1" max="1" step="0.05" class="mt-1 w-full" @change="applyAll({ opacity })" />
      </label>
      <label v-if="textLike" class="text-[11px] uppercase tracking-[0.14em] text-brass">字号
        <input v-model.number="fontSize" type="number" min="8" max="120" class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm" @change="applyAll({ fontSize })" />
      </label>
      <label v-if="textLike" class="text-[11px] uppercase tracking-[0.14em] text-brass">对齐
        <select v-model="align" class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm" @change="applyAll({ align })">
          <option value="left">左</option>
          <option value="center">中</option>
          <option value="right">右</option>
        </select>
      </label>
      <label v-if="primary?.kind === 'rect'" class="text-[11px] uppercase tracking-[0.14em] text-brass">圆角
        <input v-model.number="radius" type="range" min="0" max="48" class="mt-1 w-full" @change="applyAll({ radius })" />
      </label>
      <div class="grid grid-cols-2 gap-2 pt-1">
        <button type="button" class="rounded-lg border border-brass/25 py-1.5 text-xs" @click="ui.selectedIds.forEach((id) => doc.reorder(id, 'top'))">置顶</button>
        <button type="button" class="rounded-lg border border-brass/25 py-1.5 text-xs" @click="ui.selectedIds.forEach((id) => doc.reorder(id, 'bottom'))">置底</button>
        <button type="button" class="rounded-lg border border-brass/25 py-1.5 text-xs" @click="ui.selectedIds.forEach((id) => doc.reorder(id, 'up'))">上移</button>
        <button type="button" class="rounded-lg border border-brass/25 py-1.5 text-xs" @click="ui.selectedIds.forEach((id) => doc.reorder(id, 'down'))">下移</button>
      </div>
      <div class="flex gap-2">
        <button type="button" class="flex-1 rounded-lg bg-brass/20 py-1.5 text-xs" @click="doc.groupSelected(ui.selectedIds)">成组</button>
        <button
          type="button"
          class="flex-1 rounded-lg bg-brass/20 py-1.5 text-xs"
          @click="primary?.groupId && doc.ungroup(primary.groupId)"
        >
          解组
        </button>
      </div>
      <button type="button" class="rounded-lg bg-seal/90 py-1.5 text-xs text-paper" @click="doc.deleteShapes(ui.selectedIds)">删除</button>
    </template>
  </aside>
</template>
