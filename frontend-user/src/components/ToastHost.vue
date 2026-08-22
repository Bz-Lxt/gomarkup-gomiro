<script setup lang="ts">
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()

const tone: Record<string, string> = {
  info: 'border-brass/40',
  ok: 'border-tealink/50',
  warn: 'border-brass',
  err: 'border-seal',
}
</script>

<template>
  <div class="pointer-events-none fixed right-4 top-4 z-[70] flex w-full max-w-sm flex-col gap-2">
    <div
      v-for="t in ui.toasts"
      :key="t.id"
      class="pointer-events-auto glass-panel flex items-start gap-3 rounded-lg px-3 py-2.5 text-sm shadow-sheet"
      :class="tone[t.kind]"
    >
      <p class="min-w-0 flex-1 leading-snug">{{ t.message }}</p>
      <button type="button" class="text-brass/80 hover:text-brass" aria-label="关闭" @click="ui.dismiss(t.id)">×</button>
    </div>
  </div>
</template>
