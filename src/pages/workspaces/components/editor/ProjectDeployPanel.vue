<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDropdown, RsFormItem, RsInput, RsSelect, RsSwitch, RsTextarea, type RsSelectModelValue } from '@/ui'
import type { EnvRow } from '@/pages/envs/kinds'
import { asSelectString } from '../../api'
import {
  envKindForAction,
  newPostAction,
  normalizeWhen,
  postActionKinds,
  postWhenKinds,
  type PostAction,
  type PostActionKind,
  type PostActionWhen,
} from '../../lib/post-actions'
import type { ProjectRecipe } from '../../lib/project-defaults'

const recipe = defineModel<ProjectRecipe>('recipe', { required: true })

const props = defineProps<{
  envs: EnvRow[]
}>()

const { t } = useI18n()

const addItems = computed(() =>
  postActionKinds.map((kind) => ({ value: kind, label: t(`workspaces.postKinds.${kind}`) })),
)

const whenOptions = computed(() =>
  postWhenKinds.map((value) => ({ value, label: t(`workspaces.postWhen.${value}`) })),
)

const actions = computed({
  get: () => recipe.value.post_actions ?? [],
  set: (rows) => {
    recipe.value.post_actions = rows
  },
})

function optionsOf(kind: PostActionKind): { value: string; label: string }[] {
  const envKind = envKindForAction(kind)
  if (!envKind) return []
  return props.envs
    .filter((item) => item.env_kind === envKind && item.is_enabled !== 0)
    .map((item) => ({ value: item.rec_id, label: item.env_name }))
}

function addAction(kind: string): void {
  if (!postActionKinds.includes(kind as PostActionKind)) return
  actions.value = [...actions.value, newPostAction(kind as PostActionKind)]
}

function removeAction(id: string): void {
  actions.value = actions.value.filter((item) => item.id !== id)
}

function patch(id: string, patchValue: Partial<PostAction>): void {
  actions.value = actions.value.map((item) => (item.id === id ? { ...item, ...patchValue } : item))
}

function onEnv(id: string, value: RsSelectModelValue): void {
  patch(id, { env_rec_id: asSelectString(value) })
}

function onWhen(id: string, value: RsSelectModelValue): void {
  patch(id, { when: normalizeWhen(asSelectString(value)) })
}

function whenOf(item: PostAction): PostActionWhen {
  return normalizeWhen(item.when)
}
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <h3>{{ t('workspaces.nav.deploy') }}</h3>
      <p>{{ t('workspaces.sectionDeployDesc') }}</p>
    </header>

    <div class="path">
      <span>{{ t('workspaces.postOrder') }}</span>
      <ol v-if="actions.length" class="order">
        <li v-for="(item, i) in actions" :key="item.id">
          {{ t('workspaces.postOrderItem', { n: i + 1, kind: t(`workspaces.postKinds.${item.kind}`), when: t(`workspaces.postWhen.${whenOf(item)}`) }) }}
        </li>
      </ol>
      <span v-else>{{ t('workspaces.postEmpty') }}</span>
    </div>

    <article v-for="item in actions" :key="item.id" class="card">
      <div class="card-head">
        <h4>{{ t(`workspaces.postKinds.${item.kind}`) }}</h4>
        <RsSwitch :model-value="item.enabled !== false" @update:model-value="patch(item.id, { enabled: $event === true })" />
        <RsButton class="more" size="sm" variant="ghost" type="button" @click="removeAction(item.id)">
          {{ t('common.delete') }}
        </RsButton>
      </div>
      <p class="hint">{{ t(`workspaces.postKindHint.${item.kind}`) }}</p>

      <div class="grid2">
        <RsFormItem :label="t('workspaces.postWhenLabel')">
          <RsSelect
            :model-value="whenOf(item)"
            :options="whenOptions"
            @update:model-value="(v: RsSelectModelValue) => onWhen(item.id, v)"
          />
        </RsFormItem>
        <RsFormItem v-if="envKindForAction(item.kind)" :label="t('workspaces.envPick')">
          <RsSelect
            :model-value="item.env_rec_id"
            :options="optionsOf(item.kind)"
            clearable
            :placeholder="t('workspaces.envPick')"
            @update:model-value="(v: RsSelectModelValue) => onEnv(item.id, v)"
          />
        </RsFormItem>
      </div>
      <p class="hint">{{ t(`workspaces.postWhenHint.${whenOf(item)}`) }}</p>

      <RsInput
        v-if="['ftp', 'sftp', 'ssh'].includes(item.kind)"
        :model-value="item.remote_path"
        :label="t('workspaces.remotePath')"
        :placeholder="t('workspaces.remotePathPh')"
        @update:model-value="patch(item.id, { remote_path: $event })"
      />

      <template v-if="item.kind === 'email'">
        <RsInput
          :model-value="item.to"
          :label="t('workspaces.mailTo')"
          :placeholder="t('workspaces.mailToPh')"
          @update:model-value="patch(item.id, { to: $event })"
        />
        <p class="hint">{{ t('workspaces.mailToHint') }}</p>
        <RsInput
          :model-value="item.subject"
          :label="t('workspaces.mailSubject')"
          :placeholder="t('workspaces.mailSubjectPh')"
          @update:model-value="patch(item.id, { subject: $event })"
        />
        <RsFormItem :label="t('workspaces.mailAttachLog')">
          <RsSwitch
            :model-value="item.attach_log === true"
            @update:model-value="patch(item.id, { attach_log: $event === true })"
          />
        </RsFormItem>
        <p class="hint">{{ t('workspaces.mailAttachLogHint') }}</p>
      </template>

      <RsTextarea
        v-if="item.kind === 'shell'"
        :model-value="item.command"
        :label="t('workspaces.shellCommand')"
        :placeholder="t('workspaces.shellCommandPh')"
        :rows="3"
        @update:model-value="patch(item.id, { command: $event })"
      />
    </article>

    <RsDropdown
      :items="addItems"
      :placeholder="t('workspaces.addPostAction')"
      :show-selected="false"
      @select="addAction"
    />
    <p class="hint">{{ t('workspaces.deployHint') }}</p>
  </section>
</template>

<style scoped src="./panel.css"></style>
