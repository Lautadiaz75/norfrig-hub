export interface Photo {
  id: number
  path: string
  filename: string
  root: string
  sku_primary: string
  size_bytes: number
  mtime: string
  indexed_at: string
  thumb_url: string
  full_url: string
}

export interface SearchResponse {
  results: Photo[]
  total: number
  took_ms: number
}

export interface Stats {
  total_photos: number
  indexed_roots: string[]
  last_indexed: string
}
