<script setup lang="ts">
import { ref, watch } from 'vue'
import { useSessionStore } from '@/stores/session'
import { useUiStore } from '@/stores/ui'

const session = useSessionStore()
const ui = useUiStore()
const nick = ref(session.nickname)
const col = ref(session.color)
const palette = ['#c45c26', '#2f5d56', '#9b2c2c', '#3d5a80', '#c4a35a', '#4a7c59', '#8b5a2b', '#9a3412']
const err = ref('')

watch(
  () => ui.nickOpen,
  (o) => {
    if (o) {
      nick.value = session.nickname
      col.value = session.color
      err.value = ''
    }
  },
)

function save() {
  const n = nick.value.trim()
  if (!n) {
    err.value = '请填写昵称'
    return
  }
  if (n.length > 24) {
    err.value = '昵称最多 24 字'
    return
  }
  session.persistIdentity(n, col.value)
  ui.nickOpen = false
}
</script>

<template>
  <div
    v-if="ui.nickOpen"
    class="fixed inset-0 z-[76] flex items-center justify-center bg-dusk/55 px-4 backdrop-blur-sm"
    @click.self="ui.nickOpen = false"
  >
    <div class="glass-panel w-full max-w-md rounded-2xl p-6">
      <p class="font-display text-xl">签署制图员名牌</p>
      <p class="mt-1 text-sm opacity-70">匿名身份保存在本机，用于协同光标与成员条。</p>
      <label class="mt-5 block text-xs uppercase tracking-[0.16em] text-brass">
        昵称 *
        <input v-model="nick" maxlength="24" class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm outline-none focus:border-brass" />
      </label>
      <p v-if="err" class="mt-1 text-xs text-seal">{{ err }}</p>
      <p class="mt-4 text-xs uppercase tracking-[0.16em] text-brass">用户色</p>
      <div class="mt-2 flex flex-wrap gap-2">
        <button
          v-for="c in palette"
          :key="c"
          type="button"
          class="h-8 w-8 rounded-full border-2"
          :class="col === c ? 'border-brass scale-110' : 'border-transparent'"
          :style="{ background: c }"
          :aria-label="c"
          @click="col = c"
        />
      </div>
      <div class="mt-6 flex justify-end">
        <button type="button" class="rounded-full bg-brass px-5 py-2 text-sm font-medium text-ink" @click="save">进入工作室</button>
      </div>
    </div>
  </div>
</template>
