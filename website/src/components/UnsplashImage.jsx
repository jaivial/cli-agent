const DEFAULT_WIDTHS = [480, 768, 1024, 1280, 1600]

function buildUrl(id, w, q = 80) {
  return `https://images.unsplash.com/photo-${id}?auto=format&fit=crop&w=${w}&q=${q}`
}

export default function UnsplashImage({
  id,
  alt,
  className,
  sizes = '(max-width: 980px) 100vw, 520px',
  priority = false,
  quality = 80,
}) {
  if (!id) return null

  const src = buildUrl(id, 1280, quality)
  const srcSet = DEFAULT_WIDTHS.map(w => `${buildUrl(id, w, quality)} ${w}w`).join(', ')

  return (
    <img
      className={className}
      src={src}
      srcSet={srcSet}
      sizes={sizes}
      alt={alt || ''}
      loading={priority ? 'eager' : 'lazy'}
      decoding="async"
      fetchpriority={priority ? 'high' : undefined}
    />
  )
}

