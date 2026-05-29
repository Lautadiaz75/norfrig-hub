import { useEffect } from 'react'
import type { Photo } from '../types'
import { rootDotColor } from './FilterBar'
import { openFolder, openFile } from '../api'

interface Props {
  photo: Photo
  index: number
  total: number
  onClose: () => void
  onPrev: () => void
  onNext: () => void
}

export function PhotoModal({ photo, index, total, onClose, onPrev, onNext }: Props) {
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if ((e.target as HTMLElement).dataset.overlay) onClose()
    }
    window.addEventListener('click', handler)
    return () => window.removeEventListener('click', handler)
  }, [onClose])

  const sizeMB = (photo.size_bytes / 1024 / 1024).toFixed(1)
  const fecha = new Date(photo.mtime).toLocaleDateString('es-AR', {
    day: '2-digit', month: '2-digit', year: 'numeric',
  })

  return (
    <div
      data-overlay="1"
      className="fixed inset-0 z-50 flex items-center justify-center p-6"
      style={{
        background: 'rgba(0,0,0,0.65)',
        backdropFilter: 'blur(30px)',
        WebkitBackdropFilter: 'blur(30px)',
      }}
    >
      <div
        className="relative flex max-w-5xl w-full overflow-hidden"
        style={{
          background: '#2c2c2e',
          borderRadius: '20px',
          boxShadow: '0 40px 100px rgba(0,0,0,0.9)',
          border: '1px solid rgba(255,255,255,0.1)',
          maxHeight: '88vh',
        }}
        onClick={e => e.stopPropagation()}
      >
        {/* close */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 z-10 w-8 h-8 rounded-full flex items-center justify-center
                     transition-colors hover:bg-white/10"
          style={{ color: 'rgba(235,235,245,0.5)' }}
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        {/* imagen */}
        <div className="flex-1 flex items-center justify-center relative"
          style={{ background: '#1c1c1e', minWidth: 0, minHeight: '400px' }}>
          <img
            src={photo.thumb_url}
            alt={photo.sku_primary}
            className="object-contain"
            style={{ maxHeight: '88vh', maxWidth: '100%', padding: '28px' }}
          />
          {index > 0 && <NavButton direction="prev" onClick={onPrev} />}
          {index < total - 1 && <NavButton direction="next" onClick={onNext} />}
          <span
            className="absolute bottom-4 left-1/2 -translate-x-1/2 px-3 py-1 rounded-full"
            style={{
              background: 'rgba(44,44,46,0.85)',
              color: 'rgba(235,235,245,0.5)',
              fontSize: '11px',
              backdropFilter: 'blur(8px)',
            }}
          >
            {index + 1} / {total}
          </span>
        </div>

        {/* metadata */}
        <div className="w-72 flex-shrink-0 flex flex-col overflow-y-auto"
          style={{ borderLeft: '1px solid rgba(255,255,255,0.08)', padding: '28px 24px' }}>
          <div className="flex items-center gap-2 mb-5 mt-6">
            <span className={`w-2 h-2 rounded-full ${rootDotColor(photo.root)}`} />
            <span className="text-xs font-medium" style={{ color: 'rgba(235,235,245,0.45)' }}>
              {photo.root}
            </span>
          </div>

          <p className="text-2xl font-bold tracking-tight text-white mb-5">
            {photo.sku_primary || '—'}
          </p>

          <div style={{ height: '1px', background: 'rgba(255,255,255,0.08)', marginBottom: '20px' }} />

          <div className="flex flex-col gap-4 flex-1">
            <Field label="Archivo" value={photo.filename} mono />
            <Field label="Ruta" value={photo.path} mono small />
            <div className="grid grid-cols-2 gap-4">
              <Field label="Fecha" value={fecha} />
              <Field label="Tamaño" value={`${sizeMB} MB`} />
            </div>
          </div>

          {/* acciones nativas */}
          <div className="flex flex-col gap-2 mt-6">
            <button
              onClick={() => openFolder(photo.id)}
              className="w-full py-2.5 rounded-xl text-sm font-medium transition-opacity hover:opacity-80"
              style={{ background: '#0a84ff', color: '#fff' }}
            >
              Abrir carpeta
            </button>
            <button
              onClick={() => openFile(photo.id)}
              className="w-full py-2.5 rounded-xl text-sm font-medium transition-opacity hover:opacity-80"
              style={{ background: 'rgba(118,118,128,0.24)', color: 'rgba(235,235,245,0.8)' }}
            >
              Ver original
            </button>
            <button
              onClick={() => navigator.clipboard.writeText(photo.path)}
              className="w-full py-2.5 rounded-xl text-sm font-medium transition-opacity hover:opacity-80"
              style={{ background: 'rgba(118,118,128,0.24)', color: 'rgba(235,235,245,0.8)' }}
            >
              Copiar ruta
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function NavButton({ direction, onClick }: { direction: 'prev' | 'next'; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="absolute top-1/2 -translate-y-1/2 w-9 h-9 rounded-full flex items-center justify-center
                 transition-colors hover:bg-white/20"
      style={{
        [direction === 'prev' ? 'left' : 'right']: '12px',
        background: 'rgba(44,44,46,0.85)',
        color: 'rgba(235,235,245,0.8)',
        border: '1px solid rgba(255,255,255,0.1)',
        backdropFilter: 'blur(8px)',
      }}
    >
      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        {direction === 'prev'
          ? <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          : <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
        }
      </svg>
    </button>
  )
}

function Field({ label, value, mono, small }: { label: string; value: string; mono?: boolean; small?: boolean }) {
  return (
    <div>
      <p className="uppercase tracking-widest font-medium mb-1"
        style={{ fontSize: '9px', color: 'rgba(235,235,245,0.35)' }}>
        {label}
      </p>
      <p className={`leading-snug break-all ${mono ? 'font-mono' : ''} ${small ? 'text-xs' : 'text-sm'}`}
        style={{ color: 'rgba(235,235,245,0.8)' }}
        data-selectable>
        {value}
      </p>
    </div>
  )
}
