export default function BrandMark({ label = 'CV', subtitle = 'Early Years' }) {
  return (
    <div className="brand-mark" aria-label={`${label} — ${subtitle}`}>
      <div className="brand-stamp">
        <span className="brand-stamp-text">{label}</span>
      </div>
      <div className="brand-meta">
        <span className="brand-meta-title">{subtitle}</span>
        <span className="brand-meta-code">(cv.site)</span>
      </div>
    </div>
  )
}

