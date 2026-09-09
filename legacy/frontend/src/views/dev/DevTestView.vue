<script setup lang="ts">
import { ref } from 'vue'
import DialogTestPanel from './DialogTestPanel.vue'
import { useMessage } from '@/components/feedback/message'

const message = useMessage()
const text = ref('dev route is working')

function notify(): void {
  message.success(`Test message: ${text.value}`)
}
</script>

<template>
  <div class="dev-test">
    <el-card class="dev-test__card">
      <template #header>Message 测试</template>
      <el-space direction="vertical" fill>
        <el-input v-model="text" />
        <el-button type="primary" @click="notify">触发 Message</el-button>
      </el-space>
    </el-card>

    <DialogTestPanel />
  </div>
</template>

<style scoped>
.dev-test {
  max-width: 720px;
  margin: 24px auto;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.dev-test__card {
  width: 100%;
}
</style>
