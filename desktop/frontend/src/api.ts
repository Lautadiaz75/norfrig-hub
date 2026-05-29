import type { Photo } from './types'

export interface SearchResult {
  results: Photo[]
  total: number
  took_ms: number
}

// Wails inyecta window.go con los métodos del struct App de Go
declare global {
  interface Window {
    go?: {
      main: {
        App: {
          Search(q: string, root: string, limit: number): Promise<SearchResult>
          Stats(): Promise<{ total_photos: number; indexed_roots: string[] }>
          OpenFolder(id: number): Promise<void>
          OpenFile(id: number): Promise<void>
          Reindex(): Promise<void>
        }
      }
    }
  }
}

export const search = (q: string, root: string, limit: number) =>
  window.go!.main.App.Search(q, root, limit)

export const getStats = () =>
  window.go!.main.App.Stats()

export const openFolder = (id: number) =>
  window.go!.main.App.OpenFolder(id)

export const openFile = (id: number) =>
  window.go!.main.App.OpenFile(id)

export const reindex = () =>
  window.go!.main.App.Reindex()
