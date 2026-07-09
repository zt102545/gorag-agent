<template>
  <div class="page-card">
    <el-tabs v-model="tab">
      <el-tab-pane label="Chat Model" name="chat">
        <ProviderForm kind="chat" />
      </el-tab-pane>
      <el-tab-pane label="Embedding" name="embedding">
        <ProviderForm kind="embedding" />
      </el-tab-pane>
      <el-tab-pane label="Agent" name="agent">
        <AgentConfigForm />
      </el-tab-pane>
      <el-tab-pane label="轻量规则" name="rules">
        <ChatLightRulesForm />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AgentConfigForm from '@/components/AgentConfigForm.vue'
import ChatLightRulesForm from '@/components/ChatLightRulesForm.vue'
import ProviderForm from '@/components/ProviderForm.vue'

const tab = ref('chat')
</script>
