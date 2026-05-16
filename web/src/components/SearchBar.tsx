import { forwardRef } from 'react'

interface Props {
  value: string
  onChange: (v: string) => void
  loading: boolean
}

export const SearchBar = forwardRef<HTMLInputElement, Props>(
  ({ value, onChange, loading }, ref) => {
    return (
      <div className="relative flex-1 max-w-2xl">
        <span
          className="absolute left-3.5 top-1/2 -translate-y-1/2 pointer-events-none"
          style={{ color: 'rgba(235,235,245,0.45)' }}
        >
          {loading ? (
            <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
          ) : (
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          )}
        </span>
        <input
          ref={ref}
          type="text"
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder="Buscar por SKU…"
          className="w-full rounded-xl pl-10 pr-9 py-2.5 text-sm outline-none transition-all"
          style={{
            background: 'rgba(118,118,128,0.24)',
            color: '#ffffff',
            border: '1px solid rgba(255,255,255,0.08)',
          }}
          onFocus={e => {
            e.target.style.background = 'rgba(118,118,128,0.32)'
            e.target.style.borderColor = '#0a84ff'
            e.target.style.boxShadow = '0 0 0 3px rgba(10,132,255,0.25)'
          }}
          onBlur={e => {
            e.target.style.background = 'rgba(118,118,128,0.24)'
            e.target.style.borderColor = 'rgba(255,255,255,0.08)'
            e.target.style.boxShadow = 'none'
          }}
        />
        {value && (
          <button
            onClick={() => onChange('')}
            className="absolute right-3 top-1/2 -translate-y-1/2 w-5 h-5 rounded-full
                       flex items-center justify-center"
            style={{ background: 'rgba(118,118,128,0.5)', color: '#fff' }}
          >
            <svg className="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        )}
      </div>
    )
  }
)
SearchBar.displayName = 'SearchBar'
