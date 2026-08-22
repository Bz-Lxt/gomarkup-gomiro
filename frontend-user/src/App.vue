<script setup lang="ts">
import ToastHost from '@/components/ToastHost.vue'
import Modal from '@/components/Modal.vue'
import NicknameModal from '@/components/NicknameModal.vue'
import PassModal from '@/components/PassModal.vue'
</script>

<template>
  <div class="w-full">
    <RouterView />
    <ToastHost />
    <Modal />
    <NicknameModal />
    <PassModal />
    <div class="noise-overlay" />
  </div>
</template>
