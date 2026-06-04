<script setup lang="ts">
const config = defineModel<Record<string, unknown>>('config', { required: true })
</script>

<template>
  <el-form-item label="node path">
    <el-input v-model="config.node_path" placeholder="留空用 PATH；或 node.exe / Node 安装目录" />
  </el-form-item>
  <el-form-item label="npm path">
    <el-input v-model="config.npm_path" placeholder="npm" />
  </el-form-item>
  <el-form-item label="npx path">
    <el-input v-model="config.npx_path" placeholder="npx" />
  </el-form-item>
</template>
