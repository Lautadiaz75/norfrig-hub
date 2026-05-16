import type { Photo } from '../types'
import { rootDotColor } from './FilterBar'

interface Props {
  photos: Photo[]
  selectedIndex: number
  onSelect: (index: number) => void
}

export function PhotoGrid({ photos, selectedIndex, onSelect }: Props) {
  if (photos.length === 0) return null

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-1">
      {photos.map((photo, i) => (
        <PhotoCard
          key={photo.id}
          photo={photo}
          selected={i === selectedIndex}
          onClick={() => onSelect(i)}
        />
      ))}
    </div>
  )
}

function PhotoCard({ photo, selected, onClick }: { photo: Photo; selected: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="group relative aspect-square overflow-hidden focus:outline-none"
      style={{
        borderRadius: '10px',
        background: '#2c2c2e',
        boxShadow: selected
          ? '0 0 0 2px #0a84ff, 0 0 0 5px rgba(10,132,255,0.3)'
          : undefined,
      }}
    >
      <img
        src={photo.thumb_url}
        alt={photo.sku_primary || photo.filename}
        loading="lazy"
        decoding="async"
        className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
        onError={e => { (e.target as HTMLImageElement).style.display = 'none' }}
      />
      {/* info siempre visible */}
      <div
        className="absolute inset-x-0 bottom-0 flex flex-col p-2"
        style={{ background: 'linear-gradient(to top, rgba(0,0,0,0.85) 0%, rgba(0,0,0,0.4) 60%, transparent 100%)' }}
      >
        <p className="text-xs font-semibold text-white truncate leading-tight">
          {photo.sku_primary || '—'}
        </p>
        <div className="flex items-center gap-1 mt-0.5">
          <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${rootDotColor(photo.root)}`} />
          <span className="truncate" style={{ fontSize: '9px', color: 'rgba(255,255,255,0.65)' }}>
            {photo.root}
          </span>
        </div>
      </div>
    </button>
  )
}
