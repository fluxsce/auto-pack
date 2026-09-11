<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RsButton, RsCard, RsContextMenu, RsIcon, RsInput, RsTag, RsTooltip } from '@/ui'
import { useWorkspaceList } from '../composables/use-workspace-list'
import BuildCommandDialog from './workspace-menu/BuildCommandDialog.vue'
import BranchCloneDialog from './workspace-menu/BranchCloneDialog.vue'
import GitBranchDialog from './workspace-menu/GitBranchDialog.vue'
import PomVersionDialog from './workspace-menu/PomVersionDialog.vue'
import PostEmailDialog from './workspace-menu/PostEmailDialog.vue'
import PostShellDialog from './workspace-menu/PostShellDialog.vue'
import WorkspaceBundleDialog from './workspace-menu/WorkspaceBundleDialog.vue'
import WorkspaceDialog from './workspace-menu/WorkspaceDialog.vue'

const { t } = useI18n()
const {
  ws,
  filter,
  busyId,
  dialogOpen,
  dialogMode,
  editing,
  filtered,
  isOwner,
  roleLabel,
  select,
  openCreate,
  openEdit,
  openClone,
  openImport,
  onMenu,
  menuItems,
  remove,
  fileInput,
  onFile,
  bundleOpen,
  bundleMode,
  bundleTarget,
  bundle,
  branchOpen,
  onGitBranchSuccess,
  cloneOpen,
  pomOpen,
  commandOpen,
  emailOpen,
  shellOpen,
  actionTarget,
} = useWorkspaceList()
</script>

<template>
  <div class="ws-pane">
    <RsCard class="ws-pane__card" fill variant="outlined" elevated radius="lg" :title="t('workspaces.listTitle')" :padding="false">
    <template #actions>
      <RsButton size="sm" variant="ghost" @click="openImport('new')">{{ t('workspaces.importWorkspace') }}</RsButton>
      <RsButton size="sm" @click="openCreate">{{ t('workspaces.newWorkspace') }}</RsButton>
    </template>
    <div class="ws-toolbar">
      <RsInput
        v-model="filter"
        clearable
        :placeholder="t('workspaces.searchPlaceholder')"
        :aria-label="t('workspaces.search')"
      >
        <template #prefix>
          <RsIcon name="search" />
        </template>
      </RsInput>
    </div>
    <div v-if="filtered.length" class="ws-list">
      <RsContextMenu
        v-for="item in filtered"
        :key="item.rec_id"
        :items="menuItems(item)"
        @select="onMenu(item, $event)"
      >
      <article
        class="ws-item"
        :class="{ 'is-active': item.rec_id === ws.currentId }"
      >
        <div class="ws-item__top">
          <button type="button" class="ws-item__main" @click="select(item)">
            <span class="ws-item__name">{{ item.workspace_name }}</span>
            <span v-if="item.workspace_code !== item.workspace_name" class="ws-item__code">{{ item.workspace_code }}</span>
          </button>
          <div class="ws-item__ops">
            <RsTooltip :content="t('workspaces.cloneWorkspace')">
              <RsButton size="sm" variant="ghost" @click.stop="openClone(item)">
                <RsIcon name="copy" />
              </RsButton>
            </RsTooltip>
            <RsTooltip v-if="isOwner(item)" :content="t('workspaces.editWorkspace')">
              <RsButton size="sm" variant="ghost" @click.stop="openEdit(item)">
                <RsIcon name="pencil" />
              </RsButton>
            </RsTooltip>
            <RsTooltip v-if="isOwner(item) && !item.is_default" :content="t('common.delete')">
              <RsButton size="sm" variant="ghost" :loading="busyId === item.rec_id" @click.stop="remove(item)">
                <RsIcon name="trash-2" />
              </RsButton>
            </RsTooltip>
          </div>
        </div>
        <button type="button" class="ws-item__meta" @click="select(item)">
          <span class="ws-item__tags">
            <RsTag v-if="item.is_default" variant="warning" round>{{ t('workspaces.defaultTag') }}</RsTag>
            <RsTag v-if="item.member_role" round>{{ roleLabel(item.member_role) }}</RsTag>
            <RsTag variant="info" round>{{ t('workspaces.projectCount', { n: item.project_count ?? 0 }) }}</RsTag>
          </span>
          <span v-if="item.root_dir" class="ws-item__root" :title="item.root_dir">{{ item.root_dir }}</span>
          <span v-if="item.remark" class="ws-item__remark">{{ item.remark }}</span>
        </button>
      </article>
      </RsContextMenu>
    </div>
      <p v-else class="page__quiet">{{ ws.list.length ? t('workspaces.emptySearch') : t('workspaces.emptyWs') }}</p>
    </RsCard>
    <WorkspaceDialog v-model:open="dialogOpen" :mode="dialogMode" :editing="editing" />
    <WorkspaceBundleDialog
      v-model:open="bundleOpen"
      :mode="bundleMode"
      :target="bundleTarget"
      :bundle="bundle"
    />
    <GitBranchDialog v-model:open="branchOpen" :workspace="actionTarget" @success="onGitBranchSuccess" />
    <BranchCloneDialog v-model:open="cloneOpen" :workspace="actionTarget" @success="onGitBranchSuccess" />
    <PomVersionDialog v-model:open="pomOpen" :workspace="actionTarget" @success="onGitBranchSuccess" />
    <BuildCommandDialog v-model:open="commandOpen" :workspace="actionTarget" @success="onGitBranchSuccess" />
    <PostEmailDialog v-model:open="emailOpen" :workspace="actionTarget" />
    <PostShellDialog v-model:open="shellOpen" :workspace="actionTarget" />
    <input
      ref="fileInput"
      type="file"
      accept="application/json,.json"
      class="ws-file"
      :aria-label="t('workspaces.importWorkspace')"
      @change="onFile"
    />
  </div>
</template>

<style scoped>
.ws-pane {
  min-width: 0;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ws-pane__card {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.ws-pane__card :deep(.rs-card__body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.ws-toolbar {
  flex-shrink: 0;
  padding: 0.75rem 0.85rem 0.15rem;
}

.ws-list {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  gap: 0.55rem;
  padding: 0.7rem 0.75rem 0.85rem;
  overflow: auto;
}

@media (max-width: 860px) {
  .ws-list {
    max-height: 22rem;
  }
}

.ws-item {
  padding: 0.75rem 0.8rem 0.7rem;
  border: 1px solid var(--shell-card-border);
  border-radius: 12px;
  background: var(--shell-inset-bg);
}

.ws-item.is-active {
  border-color: rgba(14, 165, 233, 0.45);
  background: rgba(14, 165, 233, 0.1);
  box-shadow: inset 0 0 0 1px rgba(14, 165, 233, 0.12);
}

.ws-item__top {
  display: flex;
  align-items: flex-start;
  gap: 0.4rem;
}

.ws-item__main,
.ws-item__meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.2rem;
  width: 100%;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.ws-item__name {
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--shell-title);
}

.ws-item__code {
  font-family: var(--rs-font-mono);
  font-size: 0.74rem;
  color: var(--shell-muted);
}

.ws-item__ops {
  display: inline-flex;
  flex-shrink: 0;
  gap: 0.05rem;
}

.ws-item__meta {
  margin-top: 0.55rem;
  gap: 0.4rem;
}

.ws-item__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.ws-item__root,
.ws-item__remark {
  font-size: 0.75rem;
  line-height: 1.45;
  color: var(--shell-muted);
}

.ws-item__root {
  font-family: var(--rs-font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.ws-file {
  display: none;
}
</style>
