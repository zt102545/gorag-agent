<template>
  <div class="page-card">
    <div class="toolbar"><el-button type="primary" @click="openCreate">创建接入应用</el-button></div>
    <el-alert type="info" :closable="false" class="tip" title="App Secret 仅在创建时显示一次，请保存到渠道服务端，不要放入 H5 页面。" />
    <el-table :data="items" v-loading="loading">
      <el-table-column prop="name" label="应用名称" min-width="150" />
      <el-table-column prop="app_id" label="App ID" min-width="280"><template #default="{ row }"><code>{{ row.app_id || '旧版 Token 应用' }}</code></template></el-table-column>
      <el-table-column prop="channel_id" label="限定渠道" width="140" />
      <el-table-column prop="tenant_id" label="限定租户" width="140" />
      <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column prop="last_used_at" label="最近换票" width="180" />
      <el-table-column prop="expires_at" label="应用过期时间" width="180" />
      <el-table-column prop="remark" label="备注" min-width="150" />
      <el-table-column label="操作" width="200" fixed="right"><template #default="{ row }">
        <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
        <el-button link type="warning" @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
        <el-button link type="danger" @click="onDelete(row)">删除</el-button>
      </template></el-table-column>
    </el-table>
    <el-dialog v-model="dialog" :title="editing ? '编辑接入应用' : '创建接入应用'" width="560px">
      <el-form label-width="110px">
        <el-form-item label="应用名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="限定渠道"><el-input v-model="form.channel_id" placeholder="如 h5；留空表示不限定" /></el-form-item>
        <el-form-item label="限定租户"><el-input v-model="form.tenant_id" placeholder="留空表示不限定" /></el-form-item>
        <el-form-item label="过期时间"><el-date-picker v-model="form.expires_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" placeholder="可留空" style="width:100%" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <el-alert v-if="created" type="success" :closable="false" title="应用已创建，请立即保存以下凭证">
        <template #default><div class="credential"><div>App ID：<code>{{ created.app_id }}</code></div><div>App Secret：<code>{{ created.app_secret }}</code></div><el-button size="small" @click="copyCredential">复制凭证</el-button></div></template>
      </el-alert>
      <template #footer><el-button @click="dialog=false">关闭</el-button><el-button type="primary" :loading="saving" @click="onSave">{{ editing ? '保存' : '创建' }}</el-button></template>
    </el-dialog>
  </div>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '@/api/admin'
const loading=ref(false), saving=ref(false), items=ref<any[]>([]), dialog=ref(false), editing=ref<any>(null), created=ref<any>(null)
const form=reactive({name:'',channel_id:'',tenant_id:'',expires_at:'',remark:''})
async function load(){loading.value=true;try{const {data}=await api.listCallerTokens();items.value=data||[]}finally{loading.value=false}}
function reset(){Object.assign(form,{name:'',channel_id:'',tenant_id:'',expires_at:'',remark:''})}
function openCreate(){editing.value=null;created.value=null;reset();dialog.value=true}
function openEdit(row:any){editing.value=row;created.value=null;Object.assign(form,{name:row.name||'',channel_id:row.channel_id||'',tenant_id:row.tenant_id||'',expires_at:row.expires_at||'',remark:row.remark||''});dialog.value=true}
async function onSave(){saving.value=true;try{const payload={...form,expires_at:form.expires_at||null};if(editing.value){await api.updateCallerToken(editing.value.id,payload);ElMessage.success('已保存');dialog.value=false}else{const {data}=await api.createCallerToken(payload);created.value=data;ElMessage.success('应用已创建')}await load()}finally{saving.value=false}}
async function toggle(row:any){await api.updateCallerToken(row.id,{enabled:!row.enabled});await load()}
async function onDelete(row:any){await ElMessageBox.confirm('删除后该应用签发的短期 Token 将立即失效，确认删除？','提示',{type:'warning'});await api.deleteCallerToken(row.id);await load()}
async function copyCredential(){await navigator.clipboard.writeText(`APP_ID=${created.value.app_id}\nAPP_SECRET=${created.value.app_secret}`);ElMessage.success('凭证已复制')}
onMounted(load)
</script>
<style scoped>.tip{margin-bottom:12px}.toolbar{margin-bottom:12px}.credential{line-height:2;word-break:break-all}.credential code{user-select:all}</style>
