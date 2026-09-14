<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsCodeBlock, RsDialog } from '@/ui'
import {
  PROJECT_ENV_VAR_EXAMPLES,
  PROJECT_ENV_VAR_GROUPS,
  projectEnvVarsByGroup,
} from '../lib/project-env-vars'

const open = defineModel<boolean>('open', { default: false })

const { t } = useI18n()

const exampleCode = computed(() => PROJECT_ENV_VAR_EXAMPLES.join('\n'))
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="form"
    width="lg"
    :title="t('workspaces.envVars.title')"
    :description="t('workspaces.envVars.intro')"
  >
    <template #body>
      <div class="env-vars">
        <section class="env-vars__block">
          <h4 class="env-vars__heading">{{ t('workspaces.envVars.syntax') }}</h4>
          <p class="env-vars__text">{{ t('workspaces.envVars.syntaxDesc') }}</p>
        </section>

        <section class="env-vars__block">
          <h4 class="env-vars__heading">{{ t('workspaces.envVars.where') }}</h4>
          <p class="env-vars__text">{{ t('workspaces.envVars.whereDesc') }}</p>
        </section>

        <section
          v-for="group in PROJECT_ENV_VAR_GROUPS"
          :key="group"
          class="env-vars__block"
        >
          <h4 class="env-vars__heading">{{ t(`workspaces.envVars.groups.${group}`) }}</h4>
          <dl class="env-vars__list">
            <div
              v-for="item in projectEnvVarsByGroup(group)"
              :key="item.key"
              class="env-vars__row"
            >
              <dt><code>{{ item.key }}</code></dt>
              <dd>{{ t(`workspaces.envVars.vars.${item.key}`) }}</dd>
            </div>
          </dl>
        </section>

        <section class="env-vars__block">
          <h4 class="env-vars__heading">{{ t('workspaces.envVars.extra') }}</h4>
          <p class="env-vars__text">{{ t('workspaces.envVars.extraDesc') }}</p>
        </section>

        <section class="env-vars__block env-vars__block--examples">
          <h4 class="env-vars__heading">{{ t('workspaces.envVars.examples') }}</h4>
          <RsCodeBlock class="env-vars__code" :code="exampleCode" lang="shell" />
        </section>
      </div>
    </template>
  </RsDialog>
</template>

<style scoped>
.env-vars {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.env-vars__block {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.env-vars__heading {
  margin: 0;
  font-size: 0.84rem;
  font-weight: 650;
  color: var(--shell-title);
}

.env-vars__text {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.55;
  color: var(--shell-muted);
}

.env-vars__list {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin: 0;
}

.env-vars__row {
  display: grid;
  grid-template-columns: minmax(9.5rem, 34%) minmax(0, 1fr);
  gap: 0.55rem 0.75rem;
  align-items: baseline;
  padding: 0.45rem 0.55rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.55rem;
  background: var(--shell-inset-bg);
}

.env-vars__row dt {
  margin: 0;
  font-family: var(--rs-font-mono);
  font-size: 0.76rem;
  font-weight: 650;
  color: #0369a1;
}

.env-vars__row dd {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.45;
  color: var(--shell-title);
}

.env-vars__code {
  border-radius: 0.55rem;
}

@media (max-width: 640px) {
  .env-vars__row {
    grid-template-columns: 1fr;
  }
}
</style>
