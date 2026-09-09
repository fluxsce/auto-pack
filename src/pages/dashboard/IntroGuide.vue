<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { RsButton, RsIcon } from '@/ui'
import brandMark from '@/assets/brand.png'

defineProps<{
  version?: string
}>()

const emit = defineEmits<{
  submit: []
}>()

const { t } = useI18n()
const router = useRouter()

const features = ['workspace', 'build', 'deploy', 'env'] as const

const naming = ['tag', 'branch', 'rev'] as const

const roles = [
  { id: 'cs', actions: ['a1', 'a2'] as const },
  { id: 'fix', actions: ['a1', 'a2', 'a3'] as const },
] as const

const steps = computed(() => [
  { id: 1, actions: 2, examples: 3 },
  { id: 2, actions: 2, examples: 1 },
  { id: 3, actions: 1, examples: 1 },
  { id: 4, actions: 3, examples: 2 },
])

const links = computed(() => [
  { name: 'workspaces', label: t('nav.workspaces') },
  { name: 'jobs', label: t('nav.jobs') },
  { name: 'settings', label: t('nav.settings') },
  { name: 'audit', label: t('nav.audit') },
])

function go(name: string): void {
  void router.push({ name })
}
</script>

<template>
  <div class="guide">
    <header class="guide__head">
      <div class="guide__hero">
        <img class="guide__logo" :src="brandMark" alt="" aria-hidden="true" />
        <div>
          <h1 class="guide__title">{{ t('intro.title') }}</h1>
          <p class="guide__app">{{ t('brand.appTitle') }}</p>
          <p v-if="version" class="guide__ver">v{{ version }}</p>
        </div>
      </div>
      <div class="guide__actions">
        <RsButton @click="emit('submit')">{{ t('jobs.submit') }}</RsButton>
        <RsButton variant="ghost" @click="go('workspaces')">{{ t('intro.goProjects') }}</RsButton>
      </div>
    </header>

    <p class="guide__desc">{{ t('intro.desc') }}</p>

    <section class="guide__card">
      <h2>{{ t('intro.releaseTitle') }}</h2>
      <p class="guide__lead">{{ t('intro.releaseIntro') }}</p>

      <div class="guide__box">
        <h3>{{ t('intro.namingTitle') }}</h3>
        <ul>
          <li v-for="key in naming" :key="key">{{ t(`intro.naming.${key}`) }}</li>
        </ul>
      </div>

      <div class="guide__box">
        <h3>{{ t('intro.rolesTitle') }}</h3>
        <div v-for="role in roles" :key="role.id" class="guide__role">
          <h4>{{ t(`intro.roles.${role.id}.title`) }}</h4>
          <ul>
            <li v-for="action in role.actions" :key="action">
              {{ t(`intro.roles.${role.id}.${action}`) }}
            </li>
          </ul>
        </div>
      </div>

      <ol class="guide__steps">
        <li v-for="step in steps" :key="step.id" class="guide__step">
          <span class="guide__num" aria-hidden="true">{{ step.id }}</span>
          <div>
            <h3>{{ t(`intro.steps.s${step.id}.title`) }}</h3>
            <p>{{ t(`intro.steps.s${step.id}.desc`) }}</p>
            <div class="guide__block">
              <span>{{ t('intro.actionsLabel') }}</span>
              <ul>
                <li v-for="n in step.actions" :key="n">
                  {{ t(`intro.steps.s${step.id}.a${n}`) }}
                </li>
              </ul>
            </div>
            <div class="guide__block">
              <span>{{ t('intro.examplesLabel') }}</span>
              <ul class="guide__ex">
                <li v-for="n in step.examples" :key="n">
                  <code>{{ t(`intro.steps.s${step.id}.ex${n}`) }}</code>
                </li>
              </ul>
            </div>
          </div>
        </li>
      </ol>
    </section>

    <section class="guide__card">
      <h2>{{ t('intro.featuresTitle') }}</h2>
      <ul class="guide__features">
        <li v-for="key in features" :key="key">{{ t(`intro.features.${key}`) }}</li>
      </ul>
    </section>

    <section class="guide__card">
      <h2>{{ t('intro.quickLinks') }}</h2>
      <div class="guide__links">
        <button v-for="link in links" :key="link.name" type="button" class="guide__link" @click="go(link.name)">
          <span>{{ link.label }}</span>
          <RsIcon name="chevron-right" />
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.guide {
  display: grid;
  gap: 1rem;
}

.guide__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.guide__hero {
  display: flex;
  align-items: center;
  gap: 1rem;
  min-width: 0;
}

.guide__logo {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  object-fit: cover;
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.22);
}

.guide__title {
  margin: 0 0 4px;
  font-size: 1.4rem;
  font-weight: 750;
  letter-spacing: -0.02em;
  color: var(--shell-title);
}

.guide__app,
.guide__ver,
.guide__desc,
.guide__lead {
  margin: 0;
  color: var(--shell-muted);
}

.guide__app {
  font-size: 0.9rem;
}

.guide__ver {
  margin-top: 4px;
  font-size: 0.75rem;
}

.guide__desc,
.guide__lead {
  max-width: 58em;
  font-size: 0.9rem;
  line-height: 1.65;
}

.guide__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
}

.guide__card,
.guide__box {
  border: 1px solid var(--shell-card-border);
  background: var(--shell-card-bg);
  box-shadow: var(--shell-card-shadow);
  backdrop-filter: blur(16px);
}

.guide__card {
  padding: 1.15rem 1.25rem;
  border-radius: 16px;
}

.guide__card h2 {
  margin: 0 0 0.7rem;
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--shell-title);
}

.guide__box {
  margin: 0.9rem 0;
  padding: 0.8rem 0.95rem;
  border-radius: 10px;
  border-style: dashed;
  background: var(--shell-inset-bg);
}

.guide__box h3,
.guide__role h4,
.guide__step h3 {
  margin: 0 0 0.45rem;
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--shell-title);
}

.guide__box ul,
.guide__role ul,
.guide__block ul,
.guide__features {
  margin: 0;
  padding-left: 1.15em;
  font-size: 0.84rem;
  line-height: 1.6;
  color: var(--shell-muted);
}

.guide__role + .guide__role {
  margin-top: 0.7rem;
}

.guide__steps {
  margin: 0;
  padding: 0;
  list-style: none;
}

.guide__step {
  display: flex;
  gap: 0.85rem;
  padding: 0.95rem 0;
  border-top: 1px solid var(--shell-card-border);
}

.guide__step:first-child {
  padding-top: 0;
  border-top: none;
}

.guide__step p {
  margin: 0 0 0.65rem;
  font-size: 0.84rem;
  line-height: 1.6;
  color: var(--shell-muted);
}

.guide__num {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  font-size: 0.8rem;
  font-weight: 700;
  color: #0284c7;
  background: rgba(14, 165, 233, 0.12);
}

.guide__block {
  margin-bottom: 0.55rem;
}

.guide__block span {
  display: block;
  margin-bottom: 0.3rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: #0284c7;
}

.guide__ex {
  padding-left: 0;
  list-style: none;
}

.guide__ex code {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 0.75rem;
  color: var(--shell-title);
  background: var(--shell-inset-bg);
  border: 1px solid var(--shell-card-border);
}

.guide__features li + li {
  margin-top: 0.45rem;
}

.guide__links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.guide__link {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.5rem 0.9rem;
  border-radius: 8px;
  border: 1px solid var(--shell-card-border);
  background: var(--shell-inset-bg);
  color: #0284c7;
  font-size: 0.88rem;
  font-weight: 600;
  cursor: pointer;
}

.guide__link:hover {
  border-color: #0ea5e9;
  background: rgba(14, 165, 233, 0.08);
}

@media (max-width: 720px) {
  .guide__head {
    flex-direction: column;
  }
}
</style>
