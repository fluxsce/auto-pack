<script setup lang="ts">
import { RsConfigProvider, RsToaster, RsTooltipProvider } from '@/ui'
import { locale, theme } from '@/prefs/appearance'
</script>

<template>
  <RsConfigProvider :theme="theme" :locale="locale" theme-scope="global">
    <RsTooltipProvider>
      <div class="ap">
        <RouterView />
      </div>
      <RsToaster />
    </RsTooltipProvider>
  </RsConfigProvider>
</template>
