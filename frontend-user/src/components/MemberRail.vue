<script setup lang="ts">
import { useSessionStore } from '@/stores/session'
import { useUiStore } from '@/stores/ui'

const session = useSessionStore()
const ui = useUiStore()

function initial(n: string) {
  return (n || '?').slice(0, 1)
}
</script>

<template>
  <aside class="glass-panel fixed bottom-4 left-3 z-30 flex max-w-[70vw] items-center gap-2 overflow-x-auto rounded-2xl px-2 py-2 md:bottom-auto md:left-4 md:top-20 md:max-w-none md:w-14 md:flex-col md:px-0 md:py-3">
    <button
      type="button"
      class="flex h-9 w-9 items-center justify-center rounded-full text-xs font-display"
      :style="{ background: session.color, color: '#f3eadc' }"
      title="修改身份"
      @click="ui.nickOpen = true"
    >
      {{ initial(session.nickname) }}
    </button>
    <span class="h-px w-6 bg-brass/30" />
    <button
      v-for="m in session.others"
      :key="m.id"
      type="button"
      class="relative flex h-9 w-9 items-center justify-center rounded-full text-xs font-display"
      :style="{ background: m.color, color: '#f3eadc' }"
      :title="m.nickname"
    >
      {{ initial(m.nickname) }}
    </button>
  </aside>
</template>
