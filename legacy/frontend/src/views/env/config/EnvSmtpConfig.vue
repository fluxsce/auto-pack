<script setup lang="ts">
const config = defineModel<Record<string, unknown>>('config', { required: true })
</script>

<template>
  <el-form-item label="SMTP host" required>
    <el-input v-model="config.host" placeholder="smtp.example.com" />
  </el-form-item>
  <el-form-item label="Port">
    <el-input-number v-model="config.port" :min="1" :max="65535" />
  </el-form-item>
  <el-form-item label="From" required>
    <el-input v-model="config.from" placeholder="noreply@example.com" />
  </el-form-item>
  <el-form-item label="From name">
    <el-input v-model="config.from_name" placeholder="Auto Pack" />
  </el-form-item>
  <el-form-item label="Username">
    <el-input v-model="config.username" autocomplete="off" />
  </el-form-item>
  <el-form-item label="Password">
    <el-input v-model="config.password" type="password" show-password autocomplete="new-password" />
  </el-form-item>
  <el-form-item label="TLS (465)">
    <el-switch v-model="config.tls" />
  </el-form-item>
  <el-form-item label="STARTTLS (587)">
    <el-switch v-model="config.starttls" />
  </el-form-item>
  <el-form-item label="Timeout (sec)">
    <el-input-number v-model="config.timeout_sec" :min="5" :max="600" />
  </el-form-item>
</template>
