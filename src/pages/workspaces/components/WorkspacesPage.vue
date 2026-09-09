<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import PageFrame from '@/components/PageFrame.vue'
import WorkspaceList from './WorkspaceList.vue'

const { t } = useI18n()
</script>

<template>
  <PageFrame wide :title="t('workspaces.title')" :desc="t('workspaces.desc')">
    <div class="ws-split">
      <WorkspaceList />
      <RouterView />
    </div>
  </PageFrame>
</template>

<style scoped>
.ws-split {
  display: grid;
  grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
  gap: 1rem;
  align-items: stretch;
  min-height: min(40rem, calc(100dvh - 12rem));
  height: 100%;
}

@media (max-width: 860px) {
  .ws-split {
    grid-template-columns: 1fr;
    min-height: 0;
  }
}
</style>
