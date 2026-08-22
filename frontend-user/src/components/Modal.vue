<script setup lang="ts">
import { ref, watch } from 'vue'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
const input = ref('')

watch(
  () => ui.modal,
  (m) => {
    input.value = m?.input?.value ?? ''
  },
)

function confirm() {
  ui.closeModal(true, input.value)
}
function cancel() {
  ui.closeModal(false)
}
</script>

<template>
  <div
    v-if="ui.modal"
    class="fixed inset-0 z-[75] flex items-center justify-center bg-dusk/55 px-4 backdrop-blur-sm"
    @click.self="cancel"
  >
    <div class="glass-panel w-full max-w-md rounded-2xl p-6 shadow-sheet">
      <p class="font-display text-xl tracking-tight">{{ ui.modal.title }}</p>
      <p class="mt-2 text-sm leading-relaxed opacity-80">{{ ui.modal.message }}</p>
      <label v-if="ui.modal.input" class="mt-4 block text-xs uppercase tracking-[0.16em] text-brass">
        {{ ui.modal.input.label }}
        <input
          v-model="input"
          class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm outline-none focus:border-brass"
          :type="ui.modal.input.password ? 'password' : 'text'"
          :placeholder="ui.modal.input.placeholder"
          @keydown.enter="confirm"
        />
      </label>
      <div class="mt-6 flex justify-end gap-3">
        <button type="button" class="rounded-full px-4 py-2 text-sm opacity-70 hover:opacity-100" @click="cancel">
          {{ ui.modal.cancelLabel || '取消' }}
        </button>
        <button
          type="button"
          class="rounded-full px-5 py-2 text-sm font-medium"
          :class="ui.modal.danger ? 'bg-seal text-paper' : 'bg-brass text-ink'"
          @click="confirm"
        >
          {{ ui.modal.confirmLabel || '确认' }}
        </button>
      </div>
    </div>
  </div>
</template>
