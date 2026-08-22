<script setup lang="ts">
import { useUiStore, type Tool } from '@/stores/ui'

const emit = defineEmits<{ image: [] }>()
const ui = useUiStore()

const tools: { id: Tool; label: string; key: string; icon: string }[] = [
  { id: 'select', label: '选择', key: 'V', icon: 'M4 4 l6 14 3-7 7-3z' },
  { id: 'rect', label: '矩形', key: 'R', icon: 'M5 7 h14 v10 h-14z' },
  { id: 'ellipse', label: '椭圆', key: 'O', icon: 'M12 6 a7 5 0 1 0 0.1 0' },
  { id: 'diamond', label: '菱形', key: 'D', icon: 'M12 4 L20 12 L12 20 L4 12z' },
  { id: 'line', label: '直线', key: 'L', icon: 'M5 18 L19 6' },
  { id: 'arrow', label: '箭头', key: 'A', icon: 'M5 18 L18 6 M12 6 h6 v6' },
  { id: 'freedraw', label: '画笔', key: 'P', icon: 'M5 17 C9 9 14 14 19 6' },
  { id: 'text', label: '文字', key: 'T', icon: 'M6 7 h12 M12 7 v11' },
  { id: 'sticky', label: '便签', key: 'N', icon: 'M7 6 h10 v12 h-7 l-3-3z' },
]

function pick(id: Tool) {
  ui.tool = id
  ui.toolbarOpen = false
}
</script>

<template>
  <div>
    <button
      type="button"
      class="pill-bar fixed left-3 top-3 z-40 flex h-11 w-11 items-center justify-center rounded-full md:hidden"
      aria-label="打开工具"
      @click="ui.toolbarOpen = true"
    >
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
        <path d="M5 7h14M5 12h14M5 17h14" />
      </svg>
    </button>

    <div
      v-if="ui.toolbarOpen"
      class="fixed inset-0 z-40 bg-dusk/40 md:hidden"
      @click="ui.toolbarOpen = false"
    />

    <nav
      class="pill-bar z-40 flex gap-1 p-1.5"
      :class="
        ui.toolbarOpen
          ? 'fixed left-3 top-16 flex-col rounded-2xl md:hidden'
          : 'pointer-events-none fixed left-1/2 top-4 hidden -translate-x-1/2 rounded-full md:pointer-events-auto md:flex'
      "
    >
      <button
        v-for="t in tools"
        :key="t.id"
        type="button"
        class="flex h-10 w-10 items-center justify-center rounded-full transition pointer-events-auto"
        :class="ui.tool === t.id ? 'bg-brass text-ink' : 'hover:bg-brass/15'"
        :title="`${t.label} (${t.key})`"
        :aria-label="t.label"
        @click="pick(t.id)"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round">
          <path :d="t.icon" />
        </svg>
      </button>
      <button
        type="button"
        class="flex h-10 w-10 items-center justify-center rounded-full hover:bg-brass/15 pointer-events-auto"
        title="插入图片"
        aria-label="插入图片"
        @click="emit('image'); ui.toolbarOpen = false"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
          <rect x="4" y="6" width="16" height="12" rx="1.5" />
          <circle cx="9" cy="10" r="1.4" />
          <path d="M7 16l4-4 3 3 3-4 3 5" />
        </svg>
      </button>
    </nav>
  </div>
</template>
