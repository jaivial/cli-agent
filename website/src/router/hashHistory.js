const subscribers = new Set()

function normalize(url) {
  if (typeof url !== 'string') return '/'
  let u = url.trim()
  if (u.startsWith('#')) u = u.slice(1)
  if (u.startsWith('!/')) u = u.slice(1)
  if (!u) return '/'
  return u.startsWith('/') ? u : `/${u}`
}

function toLocation(url) {
  const normalized = normalize(url)
  const qIndex = normalized.indexOf('?')
  if (qIndex === -1) return { pathname: normalized, search: '' }
  return {
    pathname: normalized.slice(0, qIndex) || '/',
    search: normalized.slice(qIndex),
  }
}

function getHashUrl() {
  return normalize(typeof window !== 'undefined' ? window.location.hash : '/')
}

function notify() {
  const loc = toLocation(getHashUrl())
  for (const fn of subscribers) fn(loc)
}

if (typeof window !== 'undefined') {
  window.addEventListener('hashchange', notify)
}

const hashHistory = {
  get location() {
    return toLocation(getHashUrl())
  },

  push(url) {
    const normalized = normalize(url)
    window.location.hash = normalized
  },

  replace(url) {
    const normalized = normalize(url)
    const base = window.location.href.split('#')[0]
    window.history.replaceState(null, '', `${base}#${normalized}`)
    notify()
  },

  listen(fn) {
    subscribers.add(fn)
    return () => subscribers.delete(fn)
  },
}

export default hashHistory

