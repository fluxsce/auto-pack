<script setup lang="ts">
import { ref } from 'vue'
import { useMessage } from '@/components/feedback/message'

const message = useMessage()

const basicVisible = ref(false)
const compactVisible = ref(false)
const loadingVisible = ref(false)
const customFooterVisible = ref(false)
const noFooterVisible = ref(false)
const confirmLoading = ref(false)

async function onLoadingConfirm(): Promise<void> {
  confirmLoading.value = true
  await new Promise((r) => setTimeout(r, 1200))
  confirmLoading.value = false
  loadingVisible.value = false
  message.success('确认操作完成')
}

function onCustomConfirm(): void {
  customFooterVisible.value = false
  message.info('自定义底部：已执行')
}
</script>

<template>
  <el-card class="dialog-test">
    <template #header>AppDialog 组件测试</template>

    <p class="dialog-test__desc">
      默认<strong>窗口模式</strong>、高度约视口 76%（<code>initial-height</code> 默认
      <code>76vh</code>）；标题栏可切换全屏；可拖拽移动、拖拽边框缩放。
    </p>

    <el-space wrap>
      <el-button type="primary" @click="basicVisible = true">窗口对话框（默认）</el-button>
      <el-button @click="compactVisible = true">指定宽度</el-button>
      <el-button @click="loadingVisible = true">确认 Loading</el-button>
      <el-button @click="customFooterVisible = true">自定义 Footer</el-button>
      <el-button @click="noFooterVisible = true">无底部按钮</el-button>
    </el-space>

    <AppDialog v-model="basicVisible" title="窗口对话框" @confirm="basicVisible = false">
      <p>默认居中窗口、高度 76vh；标题栏可进入全屏。小表单可用 <code>initial-height="auto"</code> 随内容增高。</p>
    </AppDialog>

    <AppDialog
      v-model="compactVisible"
      title="480px 宽"
      width="480px"
      @confirm="compactVisible = false"
    >
      <p>通过 <code>width</code> 指定初始宽度；拖拽上下边框后才会锁定高度。</p>
    </AppDialog>

    <AppDialog
      v-model="loadingVisible"
      title="提交中"
      :confirm-loading="confirmLoading"
      confirm-text="提交"
      @confirm="onLoadingConfirm"
      @cancel="loadingVisible = false"
    >
      <p>点击「提交」模拟 1.2s 异步请求，期间确认按钮显示 loading。</p>
    </AppDialog>

    <AppDialog
      v-model="customFooterVisible"
      title="自定义底部"
      @cancel="customFooterVisible = false"
    >
      <p>使用 <code>#footer</code> 插槽可完全替换默认按钮区。</p>
      <template #footer>
        <div class="dialog-test__custom-footer">
          <el-button @click="customFooterVisible = false">关闭</el-button>
          <el-button type="warning" @click="onCustomConfirm">执行</el-button>
        </div>
      </template>
    </AppDialog>

    <AppDialog v-model="noFooterVisible" title="仅展示" :show-footer="false">
      <p>设置 <code>:show-footer="false"</code> 可隐藏底部按钮，适合纯信息展示。</p>
    </AppDialog>
  </el-card>
</template>

<style scoped>
.dialog-test__desc {
  margin: 0 0 1rem;
  color: var(--app-color-text-secondary);
  font-size: 0.9rem;
  line-height: 1.5;
}

.dialog-test__custom-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.625rem;
  width: 100%;
  max-width: var(--app-content-max-width, 1200px);
  margin: 0 auto;
  box-sizing: border-box;
}
</style>
