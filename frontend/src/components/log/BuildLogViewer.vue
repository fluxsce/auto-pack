<script setup lang="ts">
import { computed } from 'vue'
import {
  buildLogLineClass,
  classifyLogLine,
  splitBuildLogLines,
  type LogViewerVariant,
} from './build-log-highlight'

const props = withDefaults(
  defineProps<{
    content: string
    variant?: LogViewerVariant
  }>(),
  { variant: 'build' },
)

const lines = computed(() => splitBuildLogLines(props.content))
</script>

<template>
  <div class="build-log-viewer">
    <div
      v-for="(line, index) in lines"
      :key="index"
      class="build-log-viewer__line"
      :class="buildLogLineClass(classifyLogLine(line, variant))"
    >
      {{ line || ' ' }}
    </div>
  </div>
</template>

<style scoped lang="scss" src="./BuildLogViewer.scss"></style>
