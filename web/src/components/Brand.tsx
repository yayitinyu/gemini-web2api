export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className={compact ? 'brand brand--compact' : 'brand'}>
      <svg className="brand__mark" viewBox="0 0 40 40" aria-hidden="true">
        <defs>
          <linearGradient id="brand-signal" x1="6" y1="32" x2="34" y2="8" gradientUnits="userSpaceOnUse">
            <stop stopColor="#8b6cff" />
            <stop offset="1" stopColor="#47d8ff" />
          </linearGradient>
        </defs>
        <circle cx="20" cy="20" r="15.5" fill="none" stroke="rgba(229,234,255,.18)" />
        <path d="M7.5 25.5C13 27.8 18.5 26 21.8 20.1c3.8-6.7 7.4-8.2 10.7-7.3" fill="none" stroke="url(#brand-signal)" strokeWidth="2.2" strokeLinecap="round" />
        <circle cx="22" cy="19.7" r="2.6" fill="#f7f8ff" />
        <path d="M20 4.5c4.7 5.8 4.5 25.3 0 31" fill="none" stroke="rgba(229,234,255,.12)" />
      </svg>
      {!compact && (
        <span className="brand__type">
          <span>Gemini</span>
          <span>Web2API</span>
        </span>
      )}
    </div>
  )
}
