<script setup lang="ts">
const config = defineModel<Record<string, unknown>>('config', { required: true })
</script>

<template>
  <el-form-item label="JAVA_HOME">
    <el-input v-model="config.java_home" />
  </el-form-item>
  <el-form-item label="java path">
    <el-input v-model="config.java_path" :placeholder="$t('envJavaPathPlaceholder')" />
  </el-form-item>
</template>
