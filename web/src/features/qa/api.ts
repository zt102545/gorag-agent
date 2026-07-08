import http from '@/shared/api/http'
import type { QAFormState, QAImportResult, QAItem, QAListQuery, QAListResult } from './types'

export function listQA(params: QAListQuery) {
  return http.get<QAListResult>('/knowledge/qa', { params })
}

export function getQA(id: string) {
  return http.get<QAItem>(`/knowledge/qa/${id}`)
}

export function createQA(data: QAFormState) {
  return http.post<QAItem>('/knowledge/qa', data)
}

export function updateQA(id: string, data: QAFormState) {
  return http.put<QAItem>(`/knowledge/qa/${id}`, data)
}

export function deleteQA(id: string) {
  return http.delete(`/knowledge/qa/${id}`)
}

export function reclassifyQA(id: string) {
  return http.post<QAItem>(`/knowledge/qa/${id}/reclassify`)
}

export function importQA(file: File) {
  const form = new FormData()
  form.append('file', file)
  return http.post<QAImportResult>('/knowledge/qa/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function listCategories() {
  return http.get<string[]>('/knowledge/qa/categories')
}

export function listTags() {
  return http.get<string[]>('/knowledge/qa/tags')
}
