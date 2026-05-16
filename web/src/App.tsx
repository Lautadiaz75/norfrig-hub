import { useState, useEffect, useRef, useCallback } from 'react'
import type { Photo } from './types'
import { PhotoGrid } from './components/PhotoGrid'
import { PhotoModal } from './components/PhotoModal'
import { SearchBar } from './components/SearchBar'
import { FilterBar } from './components/FilterBar'

export default function App() {
  const [query, setQuery] = useState('')
  const [root, setRoot] = useState('')
  const [results, setResults] = useState<Photo[]>([])
  const [loading, setLoading] = useState(false)
  const [tookMs, setTookMs] = useState<number | null>(null)
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const searchRef = useRef<HTMLInputElement>(null)

  const selected = selectedIndex >= 0 ? results[selectedIndex] ?? null : null
  const hasQuery = query.trim().length > 0

  useEffect(() => {
    const q = query.trim()
    if (!q) { setResults([]); setTookMs(null); return }
    const timer = setTimeout(async () => {
      setLoading(true)
      try {
        const params = new URLSearchParams({ q, limit: '60' })
        if (root) params.set('root', root)
        const res = await fetch(`/api/search?${params}`)
        const data = await res.json()
        setResults(data.results ?? [])
        setTookMs(data.took_ms ?? null)
      } catch {
        setResults([])
      } finally {
        setLoading(false)
      }
    }, 300)
    return () => clearTimeout(timer)
  }, [query, root])

  const openPhoto = useCallback((index: number) => setSelectedIndex(index), [])
  const closeModal = useCallback(() => setSelectedIndex(-1), [])
  const goNext = useCallback(() => setSelectedIndex(i => Math.min(i + 1, results.length - 1)), [results.length])
  const goPrev = useCallback(() => setSelectedIndex(i => Math.max(i - 1, 0)), [])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const isInput = ['INPUT', 'TEXTAREA'].includes((e.target as HTMLElement).tagName)
      if (e.key === '/' && !isInput) { e.preventDefault(); searchRef.current?.focus(); return }
      if (e.key === 'Escape') {
        if (selectedIndex >= 0) closeModal()
        else searchRef.current?.blur()
        return
      }
      if (selectedIndex >= 0) {
        if (e.key === 'ArrowRight' || e.key === 'ArrowDown') { e.preventDefault(); goNext() }
        if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') { e.preventDefault(); goPrev() }
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [selectedIndex, closeModal, goNext, goPrev])

  return (
    <div className="min-h-screen" style={{ backgroundColor: '#1c1c1e' }}>
      {/* header */}
      <div
        className="sticky top-0 z-10 px-6 py-3"
        style={{
          backgroundColor: 'rgba(28,28,30,0.85)',
          backdropFilter: 'blur(20px)',
          WebkitBackdropFilter: 'blur(20px)',
          borderBottom: hasQuery ? '1px solid rgba(255,255,255,0.07)' : 'none',
        }}
      >
        <div className="px-3">
          <div className="flex items-center gap-3">
            <span className="text-xs font-semibold tracking-wide select-none flex-shrink-0"
              style={{ color: 'rgba(235,235,245,0.35)' }}>
              Hub
            </span>
            <SearchBar ref={searchRef} value={query} onChange={setQuery} loading={loading} />
          </div>
          {hasQuery && (
            <div className="mt-2 pl-8">
              <FilterBar root={root} onRootChange={setRoot} />
            </div>
          )}
        </div>
      </div>

      {/* content */}
      <div className="px-3">
        {/* estado vacío */}
        {!hasQuery && (
          <div className="flex flex-col items-center justify-center py-44 gap-3">
            <svg
              className="w-10 h-10"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}
              style={{ color: 'rgba(235,235,245,0.15)' }}
            >
              <path strokeLinecap="round" strokeLinejoin="round"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <p className="text-sm font-medium" style={{ color: 'rgba(235,235,245,0.4)' }}>
              Buscá por SKU para encontrar fotos
            </p>
            <p className="text-xs" style={{ color: 'rgba(235,235,245,0.25)' }}>
              Presioná{' '}
              <kbd
                className="px-1.5 py-0.5 rounded font-mono"
                style={{ background: 'rgba(255,255,255,0.08)', color: 'rgba(235,235,245,0.4)' }}
              >
                /
              </kbd>
              {' '}para enfocar
            </p>
          </div>
        )}

        {/* resultados */}
        {results.length > 0 && (
          <div className="pt-5 pb-12">
            <p className="text-xs mb-4" style={{ color: 'rgba(235,235,245,0.3)' }}>
              {results.length} resultado{results.length !== 1 ? 's' : ''}
              {tookMs !== null && ` · ${tookMs}ms`}
            </p>
            <PhotoGrid photos={results} selectedIndex={selectedIndex} onSelect={openPhoto} />
          </div>
        )}

        {/* sin resultados */}
        {hasQuery && !loading && results.length === 0 && (
          <div className="flex items-center justify-center py-36">
            <p className="text-sm" style={{ color: 'rgba(235,235,245,0.35)' }}>
              Sin resultados para{' '}
              <span className="font-mono" style={{ color: 'rgba(235,235,245,0.55)' }}>
                "{query}"
              </span>
            </p>
          </div>
        )}
      </div>

      {selected && (
        <PhotoModal
          photo={selected}
          index={selectedIndex}
          total={results.length}
          onClose={closeModal}
          onPrev={goPrev}
          onNext={goNext}
        />
      )}
    </div>
  )
}
