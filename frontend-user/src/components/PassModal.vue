<script setup lang="ts">
import { ref, watch } from 'vue'
import { api, reportApiError } from '@/lib/api'
import { useSessionStore } from '@/stores/session'
import { useUiStore } from '@/stores/ui'

const session = useSessionStore()
const ui = useUiStore()
const code = ref('')
const err = ref('')

watch(
  () => ui.passOpen,
  (o) => {
    if (o) {
      code.value = ''
      err.value = ''
    }
  },
)

async function unlock() {
  err.value = ''
  if (!code.value.trim()) {
    err.value = '请输入口令'
    return
  }
  try {
    await api.unlock(session.boardId, code.value)
    session.savePass(session.boardId, code.value)
    ui.passOpen = false
    session.connect(session.boardId)
  } catch (e) {
    err.value = reportApiError(e, '口令不正确')
  }
}
</script>

<template>
  <div v-if="ui.passOpen" class="fixed inset-0 z-[76] flex items-center justify-center bg-dusk/55 px-4 backdrop-blur-sm">
    <div class="glass-panel w-full max-w-md rounded-2xl p-6">
      <p class="stamp inline-block px-2 py-0.5 text-[10px]">Sealed</p>
      <p class="mt-3 font-display text-xl">此白板已加盖口令</p>
      <label class="mt-5 block text-xs uppercase tracking-[0.16em] text-brass">
        口令 *
        <input v-model="code" type="password" class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm outline-none" @keydown.enter="unlock" />
      </label>
      <p v-if="err" class="mt-1 text-xs text-seal">{{ err }}</p>
      <div class="mt-6 flex justify-end gap-3">
        <RouterLink to="/" class="rounded-full px-4 py-2 text-sm opacity-70">返回目录</RouterLink>
        <button type="button" class="rounded-full bg-brass px-5 py-2 text-sm font-medium text-ink" @click="unlock">解锁</button>
      </div>
    </div>
  </div>
</template>
