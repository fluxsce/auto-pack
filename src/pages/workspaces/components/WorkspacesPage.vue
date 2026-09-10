<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import PageFrame from '@/components/PageFrame.vue'
import WorkspaceList from './WorkspaceList.vue'

const { t } = useI18n()
</script>

<template>
  <PageFrame class="ws-page" wide :title="t('workspaces.title')" :desc="t('workspaces.desc')">
    <div class="ws-split">
      <div class="ws-split__pane">
        <WorkspaceList />
      </div>
      <div class="ws-split__pane">
        <RouterView />
      </div>
    </div>
  </PageFrame>
</template>

<style scoped>
.ws-page {
  height: 100%;
  max-height: 100%;
  min-height: 0;
  overflow: hidden;
}

.ws-page :deep(.page__stack) {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.ws-split {
  display: grid;
  grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
  gap: 1rem;
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  align-items: stretch;
}

.ws-split__pane {
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.ws-split__pane > :deep(*) {
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: 100%;
}

@media (max-width: 860px) {
  .ws-page {
    height: auto;
    max-height: none;
    overflow: visible;
  }

  .ws-page :deep(.page__stack),
  .ws-split,
  .ws-split__pane {
    height: auto;
    overflow: visible;
  }

  .ws-split {
    grid-template-columns: 1fr;
  }
}
</style>
