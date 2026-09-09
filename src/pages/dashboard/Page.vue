<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import IntroGuide from './IntroGuide.vue'

const router = useRouter()
const version = ref('')

onMounted(() => {
  void request<{ version?: string }>({ method: 'GET', url: apiPath('health') })
    .then((res) => {
      version.value = res.version?.trim() ?? ''
    })
    .catch(() => {
      version.value = ''
    })
})

function goSubmit(): void {
  void router.push({ name: 'jobs', query: { submit: '1' } })
}
</script>

<template>
  <section class="page">
    <IntroGuide :version="version" @submit="goSubmit" />
  </section>
</template>
