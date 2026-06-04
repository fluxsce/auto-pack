<script setup lang="ts">
const config = defineModel<Record<string, unknown>>('config', { required: true })
</script>

<template>
  <el-form-item label="Maven home">
    <el-input v-model="config.maven_home" placeholder="可选" />
  </el-form-item>
  <el-form-item label="mvn path">
    <el-input v-model="config.mvn_path" placeholder="mvn 或绝对路径" />
  </el-form-item>
  <el-form-item label="settings.xml">
    <el-input v-model="config.settings_xml" />
  </el-form-item>
  <el-form-item label="JAVA_HOME">
    <el-input v-model="config.java_home" placeholder="Maven 使用的 JDK" />
  </el-form-item>
</template>
