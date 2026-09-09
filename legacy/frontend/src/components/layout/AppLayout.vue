<script setup lang="ts">
import AppLayoutContent from '@/components/layout/AppLayoutContent.vue'
import AppLayoutHeader from '@/components/layout/AppLayoutHeader.vue'
import TechBackground from '@/components/layout/TechBackground.vue'
</script>

<template>
  <div class="app-layout app-shell tech-shell">
    <TechBackground />
    <AppLayoutHeader />
    <AppLayoutContent>
      <router-view />
    </AppLayoutContent>
  </div>
</template>
