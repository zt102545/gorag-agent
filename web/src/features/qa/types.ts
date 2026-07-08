export interface QAItem {
  id: string
  question: string
  answer: string
  category?: string
  tags?: string[]
  classify_status: string
  classify_error?: string
  classified_at?: string
  created_at?: string
  updated_at?: string
}

export interface QAListQuery {
  q: string
  category: string
  tag: string
  classify_status: string
  page: number
  size: number
}

export interface QAListResult {
  total: number
  items: QAItem[]
}

export interface QAFormState {
  question: string
  answer: string
  category: string
  tags: string[]
}

export interface QAImportResult {
  imported: number
  failed: number
  errors?: string[]
}
