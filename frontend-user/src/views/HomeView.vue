<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, reportApiError } from '@/lib/api'
import { formatBeijing } from '@/lib/time'
import type { BoardListItem } from '@/lib/protocol'
import { useSessionStore } from '@/stores/session'
import { useUiStore } from '@/stores/ui'

const router = useRouter()
const session = useSessionStore()
const ui = useUiStore()
const boards = ref<BoardListItem[]>([])
const loading = ref(true)
const title = ref('')
const pass = ref('')
const titleErr = ref('')

async function load() {
  loading.value = true
  try {
    boards.value = await api.listBoards()
  } catch (e) {
    ui.toast(reportApiError(e, '无法读取白板目录'), 'err')
  } finally {
    loading.value = false
  }
}

async function create() {
  titleErr.value = ''
  const t = title.value.trim()
  if (t.length > 80) {
    titleErr.value = '标题最多 80 字'
    ui.toast('标题过长', 'err')
    return
  }
  try {
    const code = pass.value.trim()
    const b = await api.createBoard(t || '未命名白板', code || undefined)
    title.value = ''
    pass.value = ''
    if (code) session.savePass(b.id, code)
    await router.push(`/board/${b.id}`)
  } catch (e) {
    ui.toast(reportApiError(e, '创建失败'), 'err')
  }
}

function rename(b: BoardListItem) {
  ui.ask({
    title: '重命名图纸',
    message: `当前名称：${b.title}`,
    confirmLabel: '保存',
    input: { label: '新标题 *', value: b.title, placeholder: '不超过 80 字' },
    onConfirm: async (v) => {
      const n = (v || '').trim()
      if (!n) {
        ui.toast('标题不能为空', 'err')
        return
      }
      if (n.length > 80) {
        ui.toast('标题最多 80 字', 'err')
        return
      }
      try {
        await api.patchBoard(b.id, { title: n })
        ui.toast('已更名', 'ok')
        await load()
      } catch (e) {
        ui.toast(reportApiError(e, '更名失败'), 'err')
      }
    },
  })
}

function setPass(b: BoardListItem) {
  ui.ask({
    title: b.hasPass ? '更换口令' : '加盖口令',
    message: '留空并确认即可清除口令。',
    confirmLabel: '写入',
    input: { label: '口令', value: '', password: true, placeholder: '可选' },
    onConfirm: async (v) => {
      try {
        if (!v?.trim()) await api.patchBoard(b.id, { clearPass: true })
        else await api.patchBoard(b.id, { passcode: v.trim() })
        ui.toast('口令已更新', 'ok')
        await load()
      } catch (e) {
        ui.toast(reportApiError(e, '口令更新失败'), 'err')
      }
    },
  })
}

function remove(b: BoardListItem) {
  ui.ask({
    title: '撕下这张图纸？',
    message: `将永久删除「${b.title}」及其全部图元，此操作不可撤销。`,
    danger: true,
    confirmLabel: '删除',
    onConfirm: async () => {
      try {
        await api.deleteBoard(b.id)
        ui.toast('已删除', 'ok')
        await load()
      } catch (e) {
        ui.toast(reportApiError(e, '删除失败'), 'err')
      }
    },
  })
}

async function share(b: BoardListItem) {
  const url = `${location.origin}/board/${b.id}`
  try {
    await navigator.clipboard.writeText(url)
    ui.toast('分享链接已复制', 'ok')
  } catch {
    ui.toast(url, 'info')
  }
}

onMounted(() => {
  if (!localStorage.getItem('gomiro.seen')) {
    ui.nickOpen = true
    localStorage.setItem('gomiro.seen', '1')
  }
  load()
})
</script>

<template>
  <div class="relative min-h-screen w-full bg-paper text-ink dark:bg-dusk dark:text-paper">
    <header class="flex w-full items-end justify-between gap-4 border-b border-brass/25 px-6 py-6 md:px-10">
      <div>
        <p class="text-[11px] uppercase tracking-[0.28em] text-brass">Atelier Blueprint</p>
        <h1 class="mt-1 font-display text-4xl tracking-tight">图纸目录</h1>
      </div>
      <div class="flex items-center gap-2">
        <button type="button" class="rounded-full border border-brass/30 px-3 py-1.5 text-xs" @click="ui.nickOpen = true">
          {{ session.nickname }}
        </button>
        <button type="button" class="rounded-full border border-brass/30 px-3 py-1.5 text-xs" @click="session.toggleTheme()">
          {{ session.theme === 'dark' ? '浅色纸' : '夜灯' }}
        </button>
      </div>
    </header>

    <section class="w-full border-b border-brass/20 px-6 py-6 md:px-10">
      <div class="grid w-full grid-cols-1 gap-3 md:grid-cols-3">
        <label class="text-[11px] uppercase tracking-[0.14em] text-brass">
          新图纸标题
          <input v-model="title" maxlength="80" class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm outline-none" placeholder="未命名白板" />
        </label>
        <label class="text-[11px] uppercase tracking-[0.14em] text-brass">
          口令（可选）
          <input v-model="pass" type="password" class="mt-1 w-full rounded-lg border border-brass/30 bg-transparent px-3 py-2 text-sm outline-none" />
        </label>
        <div class="flex items-end">
          <button type="button" class="w-full rounded-full bg-brass px-4 py-2.5 text-sm font-medium text-ink" @click="create">铺一张新纸</button>
        </div>
      </div>
      <p v-if="titleErr" class="mt-2 text-xs text-seal">{{ titleErr }}</p>
    </section>

    <main class="w-full px-6 py-8 md:px-10">
      <p v-if="loading" class="text-sm opacity-60">正在翻目录…</p>
      <p v-else-if="!boards.length" class="text-sm opacity-60">还没有图纸。左侧铺一张即可开画。</p>
      <ul v-else class="grid w-full grid-cols-1 gap-4 xs:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
        <li v-for="b in boards" :key="b.id" class="group relative overflow-hidden rounded-2xl border border-brass/25 bg-panel/40 shadow-sheet dark:bg-panel">
          <RouterLink :to="`/board/${b.id}`" class="block">
            <div class="aspect-[16/10] bg-[#1c1915]/40">
              <img v-if="b.thumbnail" :src="b.thumbnail" :alt="b.title" class="h-full w-full object-cover" />
              <div v-else class="flex h-full items-center justify-center font-display text-brass/50">BLANK</div>
            </div>
            <div class="p-4">
              <div class="flex items-start justify-between gap-2">
                <h2 class="font-display text-lg leading-tight">{{ b.title }}</h2>
                <span v-if="b.hasPass" class="stamp shrink-0 px-1.5 py-0.5 text-[9px]">Lock</span>
              </div>
              <p class="mt-2 text-[11px] opacity-55">更新 {{ formatBeijing(b.updatedAt) }}</p>
              <p class="text-[11px] opacity-45">创建 {{ formatBeijing(b.createdAt) }}</p>
            </div>
          </RouterLink>
          <div class="flex flex-wrap gap-2 px-4 pb-4">
            <button type="button" class="text-xs text-brass underline" @click="rename(b)">重命名</button>
            <button type="button" class="text-xs text-brass underline" @click="setPass(b)">口令</button>
            <button type="button" class="text-xs text-brass underline" @click="share(b)">分享</button>
            <button type="button" class="text-xs text-seal underline" @click="remove(b)">删除</button>
          </div>
        </li>
      </ul>
    </main>
  </div>
</template>
