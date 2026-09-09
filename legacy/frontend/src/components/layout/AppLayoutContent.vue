<script setup lang="ts">
/**
 * Enterprise console content shell (ByteDance / Alibaba Cloud pattern):
 * full-width workspace background + centered max-width work area.
 */
</script>

<template>
  <main id="main-content" class="app-layout-content" tabindex="-1">
    <div class="app-layout-content__workspace">
      <header v-if="$slots.head" class="app-layout-content__head">
        <slot name="head" />
      </header>
      <div class="app-layout-content__inner">
        <slot />
      </div>
    </div>
  </main>
</template>
