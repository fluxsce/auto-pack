<script setup lang="ts">
defineProps<{
  title: string
  desc?: string
  wide?: boolean
}>()
</script>

<template>
  <section class="page" :class="{ 'page--wide': wide }">
    <header class="page__head">
      <div class="page__titles">
        <h1>{{ title }}</h1>
        <p v-if="desc">{{ desc }}</p>
      </div>
      <div v-if="$slots.actions" class="page__actions">
        <slot name="actions" />
      </div>
    </header>
    <div class="page__stack">
      <slot />
    </div>
  </section>
</template>
