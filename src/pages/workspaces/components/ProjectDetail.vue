<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsCard, RsIcon, RsTag } from '@/ui'
import { typeLabel, typeVariant } from '../composables/use-project-list'
import { useProjectDetail } from '../composables/use-project-detail'
import { projectDetailIcons, projectDetailTabs } from '../lib/project-detail'
import ProjectConsolePane from './detail/ProjectConsolePane.vue'
import ProjectDepsPane from './detail/ProjectDepsPane.vue'
import ProjectFilesPane from './detail/ProjectFilesPane.vue'
import ProjectHistoryPane from './detail/ProjectHistoryPane.vue'
import ProjectStatusPane from './detail/ProjectStatusPane.vue'

const { t } = useI18n()
const {
  project,
  jobs,
  steps,
  logText,
  selectedJobId,
  selected,
  latest,
  loading,
  building,
  live,
  tab,
  recipe,
  checkoutPath,
  goTab,
  back,
  configure,
  buildNow,
  selectJob,
} = useProjectDetail()

const sections = computed(() =>
  projectDetailTabs.map((id) => ({
    id,
    title: t(`workspaces.detail.nav.${id}`),
    icon: projectDetailIcons[id],
  })),
)
</script>

<template>
  <div class="ws-pane">
    <RsCard class="detail" fill variant="outlined" elevated radius="lg" :padding="false">
      <template #header>
        <div class="detail__head">
          <h3 class="rs-card__title">{{ project?.project_name || t('workspaces.detail.title') }}</h3>
          <RsTag v-if="project" :variant="typeVariant(project.project_type)" round>{{ typeLabel(project.project_type) }}</RsTag>
          <div class="detail__ops">
            <RsButton size="sm" :loading="building" :disabled="!project || loading" @click="buildNow">
              {{ t('workspaces.buildNow') }}
            </RsButton>
            <RsButton size="sm" variant="ghost" @click="back">{{ t('common.back') }}</RsButton>
          </div>
        </div>
      </template>

      <div class="detail__layout">
        <nav class="detail__nav" :aria-label="t('workspaces.detail.title')">
          <button
            v-for="item in sections"
            :key="item.id"
            type="button"
            class="detail__nav-item"
            :class="{ 'is-active': item.id === tab }"
            @click="goTab(item.id)"
          >
            <RsIcon :name="item.icon" :size="16" />
            <span>{{ item.title }}</span>
          </button>
          <button type="button" class="detail__nav-item" @click="configure">
            <RsIcon name="settings" :size="16" />
            <span>{{ t('workspaces.detail.nav.config') }}</span>
          </button>
          <div class="detail__history">
            <ProjectHistoryPane aside :jobs="jobs" :selected-id="selectedJobId" @select="selectJob" />
          </div>
        </nav>

        <div class="detail__main">
          <ProjectStatusPane
            v-if="tab === 'status'"
            :scm-mode="recipe.scm_mode || 'git'"
            :checkout-path="checkoutPath"
            :branch="project?.repo_branch || ''"
            :repo-url="project?.repo_url || ''"
            :command="recipe.command || ''"
            :cron="project?.cron_expr || ''"
            :latest="latest"
            @console="goTab('console')"
          />
          <ProjectConsolePane
            v-else-if="tab === 'console'"
            :job="selected"
            :steps="steps"
            :log="logText"
            :live="live"
          />
          <ProjectHistoryPane
            v-else-if="tab === 'history'"
            :jobs="jobs"
            :selected-id="selectedJobId"
            @select="selectJob"
          />
          <ProjectFilesPane
            v-else-if="tab === 'files' && project"
            :project-id="project.rec_id"
            :workspace-id="project.workspace_rec_id || ''"
            :project-name="project.project_name"
            :project-type="project.project_type"
            :checkout-path="checkoutPath"
            :recipe="recipe"
          />
          <ProjectDepsPane
            v-else-if="tab === 'deps' && project"
            :project-id="project.rec_id"
          />
        </div>
      </div>
    </RsCard>
  </div>
</template>

<style scoped>
.ws-pane {
  min-width: 0;
  min-height: 0;
  height: 100%;
  max-height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.detail {
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.detail :deep(.rs-card__body) {
  display: flex;
  flex-direction: column;
  flex: 1 1 0;
  min-height: 0;
  overflow: hidden;
}

.detail__head {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.detail__ops {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin-left: auto;
}

.detail__layout {
  display: flex;
  flex: 1 1 0;
  min-height: 0;
  overflow: hidden;
  align-items: stretch;
}

.detail__nav {
  display: flex;
  flex-direction: column;
  align-self: stretch;
  flex-shrink: 0;
  width: 15.5rem;
  min-height: 0;
  overflow: hidden;
  padding: 0.55rem 0 0;
  border-right: 1px solid var(--shell-border);
  background: var(--shell-inset-bg);
}

.detail__nav-item {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  padding: 0.65rem 0.9rem;
  border: 0;
  border-left: 3px solid transparent;
  background: transparent;
  color: var(--shell-title);
  font-size: 0.86rem;
  text-align: left;
  cursor: pointer;
}

.detail__nav-item:hover {
  background: var(--shell-nav-hover);
}

.detail__nav-item.is-active {
  border-left-color: #0ea5e9;
  background: color-mix(in srgb, #0ea5e9 10%, transparent);
  font-weight: 650;
  color: #0ea5e9;
}

.detail__history {
  flex: 1;
  min-height: 0;
  margin-top: 0.35rem;
  padding: 0.7rem 0.65rem 0.75rem;
  border-top: 1px solid var(--shell-border);
  overflow: hidden;
}

.detail__main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 1.05rem 1.15rem 1rem;
  overflow: hidden;
}

@media (max-width: 720px) {
  .detail__layout {
    flex-direction: column;
  }

  .detail__history {
    display: none;
  }

  .detail__nav {
    width: auto;
    flex-direction: row;
    flex-wrap: wrap;
    border-right: 0;
    border-bottom: 1px solid var(--shell-border);
  }

  .detail__nav-item {
    width: auto;
    border-left: 0;
    border-bottom: 3px solid transparent;
  }

  .detail__nav-item.is-active {
    border-left-color: transparent;
    border-bottom-color: #0ea5e9;
  }
}
</style>
