<script setup lang="ts">
import { computed } from 'vue'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const label = computed(() => {
  if (session.status === 'connected') return '已连接'
  if (session.status === 'reconnecting') return '重连中'
  return '已断开'
})
const cls = computed(() => {
  if (session.status === 'connected') return 'bg-tealink'
  if (session.status === 'reconnecting') return 'bg-brass animate-pulse'
  return 'bg-seal'
})
</script>

<template>
  <div class="glass-panel inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs tracking-wide">
    <span class="h-2 w-2 rounded-full" :class="cls" />
    {{ label }}
  </div>
</template>
