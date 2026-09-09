<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { RsAlert, RsButton, RsIcon, RsInput } from '@/ui'
import { useAuthStore } from '@/stores/auth'
import PreferenceSwitch from '@/prefs/PreferenceSwitch.vue'
import TechBackdrop from '@/components/TechBackdrop.vue'
import brandMark from '@/assets/brand.png'

const { t } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const username = ref('admin')
const password = ref('123456')
const loading = ref(false)
const error = ref('')
const sessionExpired = computed(() => route.query.reason === 'expired')

const features = computed(() => [t('login.feature1'), t('login.feature2'), t('login.feature3')])

async function onSubmit(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    await auth.login(username.value.trim(), password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/jobs'
    await router.replace(redirect)
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('login.failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <TechBackdrop />

    <header class="login__toolbar">
      <div class="login__brand-bar">
        <img class="login__logo" :src="brandMark" alt="" aria-hidden="true" />
        <span>{{ t('brand.appTitle') }}</span>
      </div>
      <PreferenceSwitch />
    </header>

    <main class="login__main">
      <section class="login__hero" aria-labelledby="login-hero-title">
        <p class="login__eyebrow">{{ t('login.eyebrow') }}</p>
        <h1 id="login-hero-title" class="login__title">{{ t('login.title') }}</h1>
        <p class="login__lead">{{ t('login.lead') }}</p>
        <ul class="login__features">
          <li v-for="item in features" :key="item">{{ item }}</li>
        </ul>
      </section>

      <section class="login__panel" aria-labelledby="login-title">
        <div class="login__card">
          <div class="login__card-glow" aria-hidden="true" />
          <header class="login__card-head">
            <h2 id="login-title">{{ t('login.cardTitle') }}</h2>
            <p>{{ t('login.cardDesc') }}</p>
          </header>

          <form class="login__form" @submit.prevent="onSubmit">
            <RsAlert v-if="sessionExpired" type="warning">{{ t('login.expired') }}</RsAlert>
            <RsInput
              id="login-username"
              v-model="username"
              size="lg"
              name="username"
              autocomplete="username"
              :label="t('login.username')"
              :placeholder="t('login.usernamePlaceholder')"
              autofocus
            >
              <template #prefix>
                <RsIcon name="user" />
              </template>
            </RsInput>
            <RsInput
              id="login-password"
              v-model="password"
              type="password"
              size="lg"
              name="password"
              autocomplete="current-password"
              :label="t('login.password')"
              :placeholder="t('login.passwordPlaceholder')"
            >
              <template #prefix>
                <RsIcon name="lock" />
              </template>
            </RsInput>
            <RsAlert v-if="error" type="danger">{{ error }}</RsAlert>
            <RsButton
              class="login__submit"
              type="submit"
              size="lg"
              :loading="loading"
              :disabled="!username.trim() || !password"
            >
              {{ t('login.submit') }}
            </RsButton>
          </form>
        </div>
      </section>
    </main>

    <footer class="login__footer">{{ t('login.footer') }}</footer>
  </div>
</template>

<style scoped>
.login {
  --login-accent: var(--shell-accent);
  --login-text: var(--shell-title);
  --login-muted: var(--shell-muted);
  --login-card-bg: var(--shell-card-bg);
  --login-card-border: var(--shell-card-border);
  --login-card-shadow: var(--shell-card-shadow);
  --login-head-border: var(--shell-border);
  --login-head-title: var(--shell-title);
  --login-head-desc: var(--shell-muted);
  --login-field-label: var(--shell-muted);
  --login-input-bg: var(--rs-input-bg);
  --login-title-grad: var(--shell-title-grad);
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 100%;
  overflow: hidden;
  color: var(--login-text);
  background: transparent;
}

.login__toolbar,
.login__main,
.login__footer {
  position: relative;
  z-index: 1;
}

.login__toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 20px clamp(20px, 4vw, 48px);
}

.login__brand-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.login__logo {
  display: block;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
}

.login__main {
  flex: 1;
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(320px, 420px);
  align-items: center;
  gap: clamp(24px, 5vw, 64px);
  width: 100%;
  max-width: 1080px;
  margin: 0 auto;
  min-height: 0;
  padding: 0 clamp(20px, 4vw, 48px) 24px;
  overflow: auto;
}

.login__eyebrow {
  margin: 0 0 12px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.28em;
  text-transform: uppercase;
  color: var(--login-accent);
}

.login__title {
  margin: 0 0 16px;
  font-size: clamp(2rem, 4vw, 3rem);
  font-weight: 800;
  line-height: 1.15;
  background: var(--login-title-grad);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.login__lead {
  margin: 0 0 28px;
  max-width: 36ch;
  font-size: 15px;
  line-height: 1.7;
  color: var(--login-muted);
}

.login__features {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 12px;
}

.login__features li {
  position: relative;
  padding-left: 18px;
  font-size: 14px;
  color: var(--login-muted);
}

.login__features li::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0.55em;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--login-accent);
  box-shadow: 0 0 12px var(--login-accent);
}

.login__card {
  position: relative;
  padding: 36px 32px 28px;
  border-radius: 24px;
  border: 1px solid var(--login-card-border);
  background: var(--login-card-bg);
  backdrop-filter: blur(24px);
  box-shadow: var(--login-card-shadow);
}

.login__card-glow {
  position: absolute;
  inset: -2px;
  border-radius: inherit;
  background: linear-gradient(145deg, rgba(14, 165, 233, 0.2), rgba(139, 92, 246, 0.12));
  opacity: 0.6;
  filter: blur(20px);
  z-index: -1;
}

.login__card-head {
  margin-bottom: 28px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--login-head-border);
}

.login__card-head h2 {
  margin: 0 0 8px;
  font-size: 24px;
  font-weight: 700;
  color: var(--login-head-title);
}

.login__card-head p {
  margin: 0;
  font-size: 14px;
  color: var(--login-head-desc);
}

.login__form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.login__form :deep(.rs-field__label) {
  color: var(--login-field-label);
  font-weight: 600;
  font-size: 13px;
}

.login__form :deep(.rs-input),
.login__form :deep(.rs-input-group__control) {
  background: var(--login-input-bg);
  border-radius: 10px;
}

.login__submit {
  width: 100%;
  margin-top: 8px;
  height: 44px;
  font-weight: 600;
  letter-spacing: 0.04em;
  border: none;
  border-radius: 10px;
  background: linear-gradient(135deg, #0ea5e9 0%, #6366f1 100%);
  color: #fff;
  box-shadow: 0 8px 24px rgba(14, 165, 233, 0.35);
}

.login__submit:hover:not(:disabled) {
  background: linear-gradient(135deg, #0284c7 0%, #4f46e5 100%);
}

.login__footer {
  flex-shrink: 0;
  padding: 16px clamp(20px, 4vw, 48px) 28px;
  text-align: center;
  font-size: 12px;
  color: var(--login-muted);
}

@media (max-width: 900px) {
  .login__main {
    grid-template-columns: 1fr;
    max-width: 440px;
  }

  .login__hero {
    text-align: center;
  }

  .login__lead,
  .login__features {
    margin-inline: auto;
  }
}
</style>
