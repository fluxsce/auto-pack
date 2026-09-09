<script setup lang="ts">
import type { EnvType } from '@/types'

const envType = defineModel<EnvType>('envType', { required: true })
const config = defineModel<Record<string, unknown>>('config', { required: true })
</script>

<template>
  <el-form-item label="Host">
    <el-input v-model="config.host" placeholder="192.168.1.1" />
  </el-form-item>
  <el-form-item label="Port">
    <el-input-number v-model="config.port" :min="1" :max="65535" class="env-field-num" />
  </el-form-item>
  <el-form-item label="Username">
    <el-input v-model="config.username" autocomplete="off" />
  </el-form-item>
  <el-form-item label="Password">
    <el-input
      v-model="config.password"
      type="password"
      show-password
      autocomplete="new-password"
      placeholder="******"
    />
  </el-form-item>
  <el-form-item v-if="envType === 'ssh' || envType === 'sftp'" label="Private key">
    <el-input v-model="config.private_key_path" placeholder="~/.ssh/id_rsa" />
  </el-form-item>
  <el-form-item v-if="envType === 'sftp'" label="Remote path">
    <el-input v-model="config.remote_path" placeholder="/" />
  </el-form-item>
  <el-form-item v-if="envType === 'ftp'" label="Base path">
    <el-input v-model="config.base_path" placeholder="/" />
  </el-form-item>
  <el-form-item v-if="envType === 'ftp'" label="Passive">
    <el-switch v-model="config.passive" />
  </el-form-item>
  <el-form-item label="Timeout (s)">
    <el-input-number v-model="config.timeout_sec" :min="1" :max="120" class="env-field-num" />
  </el-form-item>
</template>

<style scoped>
.env-field-num {
  width: 100%;
}
</style>
