<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { resolveRedirect } from '@/router/guards'
import { User, Lock } from '@element-plus/icons-vue'
import { useMessage } from '@/components/feedback/message'
import { useSessionStore } from '@/stores'
import { login as apiLogin } from '@/request/api/auth'

const { t } = useI18n()
const message = useMessage()
const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()

const loginForm = ref({ username: '', password: '' })
const loginLoading = ref(false)

async function onLogin(): Promise<void> {
  const username = loginForm.value.username.trim()
  const password = loginForm.value.password
  if (!username || !password) {
    message.warning(t('loginRequired'))
    return
  }

  loginLoading.value = true
  try {
    const result = await apiLogin({ username, password })
    sessionStore.setUser(result.user)
    message.success(t('loginSuccess'))
    const redirect = resolveRedirect(route.query.redirect)
    await router.replace(redirect ?? { name: 'env' })
  } catch (error) {
    message.error((error as Error).message)
  } finally {
    loginLoading.value = false
  }
}
</script>

<template>
  <section class="login-view">
    <div class="login-view__hero">
      <p class="login-view__eyebrow">{{ t('loginEyebrow') }}</p>
      <h1 class="login-view__title">{{ t('loginTitle') }}</h1>
      <p class="login-view__subtitle">{{ t('loginSubtitle') }}</p>
      <ul class="login-view__features">
        <li v-for="item in 3" :key="item">{{ t(`loginFeature${item}`) }}</li>
      </ul>
    </div>

    <div class="login-view__panel">
      <div class="login-view__card">
        <div class="login-view__card-glow" aria-hidden="true" />
        <header class="login-view__card-head">
          <h2>{{ t('loginCardTitle') }}</h2>
          <p>{{ t('loginCardDesc') }}</p>
        </header>

        <el-form class="login-view__form" label-position="top" @submit.prevent="onLogin">
          <el-form-item :label="t('loginUsername')">
            <el-input
              v-model="loginForm.username"
              size="large"
              :prefix-icon="User"
              autocomplete="username"
              :placeholder="t('loginUsernamePlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="t('loginPassword')">
            <el-input
              v-model="loginForm.password"
              size="large"
              type="password"
              show-password
              :prefix-icon="Lock"
              autocomplete="current-password"
              :placeholder="t('loginPasswordPlaceholder')"
            />
          </el-form-item>
          <el-button
            class="login-view__submit"
            type="primary"
            size="large"
            :loading="loginLoading"
            native-type="submit"
          >
            {{ t('loginSubmit') }}
          </el-button>
        </el-form>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
.login-view {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(320px, 420px);
  gap: clamp(24px, 5vw, 64px);
  align-items: center;
  width: 100%;
  max-width: 1080px;
  margin: 0 auto;
  padding: 24px 0 48px;
}

.login-view__hero {
  color: var(--shell-text);
}

.login-view__eyebrow {
  margin: 0 0 12px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.28em;
  text-transform: uppercase;
  color: var(--shell-accent);
}

.login-view__title {
  margin: 0 0 16px;
  font-size: clamp(2rem, 4vw, 3rem);
  font-weight: 800;
  line-height: 1.15;
  background: linear-gradient(120deg, #fff 0%, #a5f3fc 45%, #c4b5fd 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.login-view__subtitle {
  margin: 0 0 28px;
  max-width: 36ch;
  font-size: 15px;
  line-height: 1.7;
  color: var(--shell-text-muted);
}

.login-view__features {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 12px;

  li {
    position: relative;
    padding-left: 18px;
    font-size: 14px;
    color: var(--shell-text-muted);

    &::before {
      content: '';
      position: absolute;
      left: 0;
      top: 0.55em;
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: var(--shell-accent);
      box-shadow: 0 0 12px var(--shell-accent);
    }
  }
}

.login-view__panel {
  position: relative;
}

.login-view__card {
  position: relative;
  padding: 36px 32px 28px;
  border-radius: 24px;
  border: 1px solid var(--shell-card-border);
  background: var(--shell-card-bg);
  backdrop-filter: blur(24px);
  box-shadow: var(--shell-card-shadow);
}

.login-view__card-glow {
  position: absolute;
  inset: -2px;
  border-radius: inherit;
  background: linear-gradient(145deg, rgba(14, 165, 233, 0.2), rgba(139, 92, 246, 0.12));
  opacity: 0.6;
  filter: blur(20px);
  z-index: -1;
}

.login-view__card-head {
  margin-bottom: 28px;
  padding-bottom: 20px;
  border-bottom: 1px solid #e2e8f0;

  h2 {
    margin: 0 0 8px;
    font-size: 24px;
    font-weight: 700;
    color: #0f172a;
  }

  p {
    margin: 0;
    font-size: 14px;
    color: #64748b;
  }
}

.login-view__form {
  :deep(.el-form-item__label) {
    color: #475569;
    font-weight: 600;
    font-size: 13px;
  }

  :deep(.el-input__wrapper) {
    background: #f8fafc;
    box-shadow: 0 0 0 1px #e2e8f0;
    border-radius: 10px;
    transition: box-shadow 0.2s ease;
  }

  :deep(.el-input__wrapper:hover),
  :deep(.el-input__wrapper.is-focus) {
    box-shadow: 0 0 0 1px var(--shell-accent);
  }

  :deep(.el-input__inner) {
    color: #0f172a;
  }

  :deep(.el-input__prefix .el-icon) {
    color: #94a3b8;
  }
}

.login-view__submit {
  width: 100%;
  margin-top: 8px;
  height: 44px;
  font-weight: 600;
  letter-spacing: 0.04em;
  border: none;
  border-radius: 10px;
  background: linear-gradient(135deg, #0ea5e9 0%, #6366f1 100%);
  box-shadow: 0 8px 24px rgba(14, 165, 233, 0.35);

  &:hover {
    background: linear-gradient(135deg, #0284c7 0%, #4f46e5 100%);
  }
}

:global(html:not(.dark)) .login-view__title {
  background: linear-gradient(120deg, #0f172a 0%, #0369a1 50%, #6d28d9 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

@media (max-width: 900px) {
  .login-view {
    grid-template-columns: 1fr;
    max-width: 440px;
  }

  .login-view__hero {
    text-align: center;
  }

  .login-view__subtitle,
  .login-view__features {
    margin-inline: auto;
  }
}
</style>
