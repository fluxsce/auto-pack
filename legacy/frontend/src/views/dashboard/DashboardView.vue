<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import AppBrandLogo from '@/components/layout/AppBrandLogo.vue'
import { fetchAppHealth } from '@/request/api/app'

const { t } = useI18n()
const router = useRouter()

const appVersion = ref('')

const featureKeys = [
  'dashboardIntroFeatureWorkspace',
  'dashboardIntroFeatureBuild',
  'dashboardIntroFeatureDeploy',
  'dashboardIntroFeatureEnv',
] as const

const quickLinks = computed(() => [
  { labelKey: 'projectNav', routeName: 'projects' as const },
  { labelKey: 'envNav', routeName: 'env' as const },
  { labelKey: 'opLogNav', routeName: 'operation-logs' as const },
])

const releaseNamingKeys = [
  'dashboardReleaseNamingTag',
  'dashboardReleaseNamingBranch',
  'dashboardReleaseNamingRevTag',
] as const

const releaseRoles = [
  {
    titleKey: 'dashboardReleaseRoleCsTitle',
    actionKeys: ['dashboardReleaseRoleCsAction1', 'dashboardReleaseRoleCsAction2'],
  },
  {
    titleKey: 'dashboardReleaseRoleFixTitle',
    actionKeys: [
      'dashboardReleaseRoleFixAction1',
      'dashboardReleaseRoleFixAction2',
      'dashboardReleaseRoleFixAction3',
    ],
  },
] as const

const releaseSteps = [
  {
    titleKey: 'dashboardReleaseStep1Title',
    descKey: 'dashboardReleaseStep1Desc',
    actionKeys: ['dashboardReleaseStep1Action1', 'dashboardReleaseStep1Action2'],
    exampleKeys: ['dashboardReleaseStep1Ex1', 'dashboardReleaseStep1Ex2', 'dashboardReleaseStep1Ex3'],
  },
  {
    titleKey: 'dashboardReleaseStep2Title',
    descKey: 'dashboardReleaseStep2Desc',
    actionKeys: ['dashboardReleaseStep2Action1', 'dashboardReleaseStep2Action2'],
    exampleKeys: ['dashboardReleaseStep2Ex1'],
  },
  {
    titleKey: 'dashboardReleaseStep3Title',
    descKey: 'dashboardReleaseStep3Desc',
    actionKeys: ['dashboardReleaseStep3Action1'],
    exampleKeys: ['dashboardReleaseStep3Ex1'],
  },
  {
    titleKey: 'dashboardReleaseStep4Title',
    descKey: 'dashboardReleaseStep4Desc',
    actionKeys: ['dashboardReleaseStep4Action1', 'dashboardReleaseStep4Action2', 'dashboardReleaseStep4Action3'],
    exampleKeys: ['dashboardReleaseStep4Ex1', 'dashboardReleaseStep4Ex2'],
  },
] as const

function goTo(routeName: string): void {
  void router.push({ name: routeName })
}

function goProjects(): void {
  void router.push({ name: 'projects' })
}

onMounted(() => {
  void fetchAppHealth()
    .then((res) => {
      appVersion.value = res.version?.trim() ?? ''
    })
    .catch(() => {
      appVersion.value = ''
    })
})
</script>

<template>
  <div class="dashboard-view">
    <header class="dashboard-view__head">
      <div class="dashboard-view__hero">
        <AppBrandLogo class="dashboard-view__logo" />
        <div class="dashboard-view__hero-text">
          <h1 class="dashboard-view__title">{{ t('dashboardIntroTitle') }}</h1>
          <p class="dashboard-view__app-name">{{ t('appTitle') }}</p>
          <p v-if="appVersion" class="dashboard-view__version">v{{ appVersion }}</p>
        </div>
      </div>
      <div class="dashboard-view__actions">
        <el-button type="primary" @click="goProjects">
          {{ t('dashboardIntroGoProjects') }}
        </el-button>
      </div>
    </header>

    <p class="dashboard-view__desc">{{ t('dashboardIntroDesc') }}</p>

    <section class="dashboard-view__card">
      <h2 class="dashboard-view__section-title">{{ t('dashboardReleaseNotesTitle') }}</h2>
      <p class="dashboard-view__release-intro">{{ t('dashboardReleaseNotesIntro') }}</p>
      <div class="dashboard-view__release-naming">
        <h3 class="dashboard-view__release-naming-title">{{ t('dashboardReleaseNamingTitle') }}</h3>
        <ul class="dashboard-view__release-naming-list">
          <li v-for="key in releaseNamingKeys" :key="key">{{ t(key) }}</li>
        </ul>
      </div>
      <div class="dashboard-view__release-roles">
        <h3 class="dashboard-view__release-roles-title">{{ t('dashboardReleaseRolesTitle') }}</h3>
        <div v-for="role in releaseRoles" :key="role.titleKey" class="dashboard-view__release-role">
          <h4 class="dashboard-view__release-role-title">{{ t(role.titleKey) }}</h4>
          <ul class="dashboard-view__release-role-list">
            <li v-for="actionKey in role.actionKeys" :key="actionKey">{{ t(actionKey) }}</li>
          </ul>
        </div>
      </div>
      <ol class="dashboard-view__release-steps">
        <li v-for="(step, index) in releaseSteps" :key="step.titleKey" class="dashboard-view__release-step">
          <span class="dashboard-view__release-step-num" aria-hidden="true">{{ index + 1 }}</span>
          <div class="dashboard-view__release-step-body">
            <h3 class="dashboard-view__release-step-title">{{ t(step.titleKey) }}</h3>
            <p class="dashboard-view__release-step-desc">{{ t(step.descKey) }}</p>
            <div v-if="step.actionKeys.length" class="dashboard-view__release-block">
              <span class="dashboard-view__release-block-label">{{ t('dashboardReleaseStepActionsLabel') }}</span>
              <ul class="dashboard-view__release-actions">
                <li v-for="actionKey in step.actionKeys" :key="actionKey">{{ t(actionKey) }}</li>
              </ul>
            </div>
            <div v-if="step.exampleKeys.length" class="dashboard-view__release-block">
              <span class="dashboard-view__release-block-label">{{ t('dashboardReleaseStepExamplesLabel') }}</span>
              <ul class="dashboard-view__release-examples">
                <li v-for="exKey in step.exampleKeys" :key="exKey">
                  <code>{{ t(exKey) }}</code>
                </li>
              </ul>
            </div>
          </div>
        </li>
      </ol>
    </section>

    <section class="dashboard-view__card">
      <h2 class="dashboard-view__section-title">{{ t('dashboardIntroFeaturesTitle') }}</h2>
      <ul class="dashboard-view__list">
        <li v-for="key in featureKeys" :key="key">{{ t(key) }}</li>
      </ul>
    </section>

    <section class="dashboard-view__card">
      <h2 class="dashboard-view__section-title">{{ t('dashboardIntroQuickLinks') }}</h2>
      <div class="dashboard-view__links">
        <button
          v-for="link in quickLinks"
          :key="link.routeName"
          type="button"
          class="dashboard-view__link"
          @click="goTo(link.routeName)"
        >
          <span>{{ t(link.labelKey) }}</span>
          <el-icon :size="14"><ArrowRight /></el-icon>
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped lang="scss">
.dashboard-view {
  max-width: var(--app-content-max-width, 1200px);
  margin-inline: auto;

  &__head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 16px;
  }

  &__hero {
    display: flex;
    align-items: center;
    gap: 16px;
    min-width: 0;
  }

  &__logo {
    flex-shrink: 0;
  }

  &__title {
    margin: 0 0 4px;
    font-size: 22px;
    font-weight: 600;
    line-height: 1.35;
    letter-spacing: -0.02em;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__app-name {
    margin: 0;
    font-size: 14px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__version {
    margin: 4px 0 0;
    font-size: 12px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__actions {
    flex-shrink: 0;
    padding-top: 2px;
  }

  &__desc {
    margin: 0 0 20px;
    max-width: 52em;
    font-size: 14px;
    line-height: 1.65;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));
  }

  &__card {
    padding: 18px 20px;
    margin-bottom: 14px;
    border: 1px solid var(--shell-card-border, var(--el-border-color-lighter));
    border-radius: 12px;
    background: var(--shell-card-bg, var(--el-bg-color));
    box-shadow: var(--shell-card-shadow, none);

    &:last-child {
      margin-bottom: 0;
    }
  }

  &__section-title {
    margin: 0 0 10px;
    font-size: 14px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__release-intro {
    margin: 0 0 14px;
    font-size: 14px;
    line-height: 1.6;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));
  }

  &__release-naming {
    margin-bottom: 18px;
    padding: 12px 14px;
    border-radius: 8px;
    background: var(--shell-prefs-bg, var(--el-fill-color-light));
    border: 1px dashed var(--shell-card-border, var(--el-border-color-lighter));
  }

  &__release-naming-title {
    margin: 0 0 8px;
    font-size: 13px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__release-naming-list {
    margin: 0;
    padding-left: 1.2em;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));

    li + li {
      margin-top: 4px;
    }
  }

  &__release-roles {
    margin-bottom: 18px;
    padding: 12px 14px;
    border-radius: 8px;
    background: var(--shell-prefs-bg, var(--el-fill-color-light));
    border: 1px dashed var(--shell-card-border, var(--el-border-color-lighter));
  }

  &__release-roles-title {
    margin: 0 0 10px;
    font-size: 13px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__release-role {
    margin-bottom: 10px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  &__release-role-title {
    margin: 0 0 6px;
    font-size: 13px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__release-role-list {
    margin: 0;
    padding-left: 1.2em;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));

    li + li {
      margin-top: 4px;
    }
  }

  &__release-steps {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  &__release-step {
    display: flex;
    gap: 12px;
    padding: 14px 0;
    border-top: 1px solid var(--shell-card-border, var(--el-border-color-lighter));

    &:first-child {
      padding-top: 0;
      border-top: none;
    }

    &:last-child {
      padding-bottom: 0;
    }
  }

  &__release-step-num {
    flex-shrink: 0;
    width: 26px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    font-size: 13px;
    font-weight: 600;
    color: var(--shell-accent, var(--el-color-primary));
    background: rgba(14, 165, 233, 0.1);
  }

  &__release-step-body {
    flex: 1;
    min-width: 0;
  }

  &__release-step-title {
    margin: 0 0 6px;
    font-size: 14px;
    font-weight: 600;
    line-height: 1.4;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__release-step-desc {
    margin: 0 0 10px;
    font-size: 13px;
    line-height: 1.6;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));
  }

  &__release-block {
    margin-bottom: 8px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  &__release-block-label {
    display: block;
    margin-bottom: 4px;
    font-size: 12px;
    font-weight: 600;
    color: var(--shell-accent, var(--el-color-primary));
  }

  &__release-actions {
    margin: 0;
    padding-left: 1.2em;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));

    li + li {
      margin-top: 4px;
    }
  }

  &__release-examples {
    margin: 0;
    padding: 0;
    list-style: none;

    li + li {
      margin-top: 4px;
    }

    code {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 6px;
      font-size: 12px;
      font-family: ui-monospace, 'Cascadia Code', 'Consolas', monospace;
      color: var(--shell-content-text, var(--el-text-color-primary));
      background: var(--shell-prefs-bg, var(--el-fill-color-light));
      border: 1px solid var(--shell-card-border, var(--el-border-color-lighter));
    }
  }

  &__list {
    margin: 0;
    padding-left: 1.2em;
    font-size: 14px;
    line-height: 1.6;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));

    li + li {
      margin-top: 8px;
    }
  }

  &__links {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  &__link {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border: 1px solid var(--shell-card-border, var(--el-border-color-lighter));
    border-radius: 8px;
    background: var(--shell-prefs-bg, var(--el-fill-color-blank));
    color: var(--shell-accent, var(--el-color-primary));
    font-size: 14px;
    cursor: pointer;
    transition:
      border-color 0.15s ease,
      background 0.15s ease;

    &:hover {
      border-color: var(--shell-accent, var(--el-color-primary));
      background: rgba(14, 165, 233, 0.06);
    }
  }
}

@media (max-width: 640px) {
  .dashboard-view__head {
    flex-direction: column;
    align-items: stretch;
  }

  .dashboard-view__actions {
    padding-top: 0;

    .el-button {
      width: 100%;
    }
  }
}
</style>
