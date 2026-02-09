import { useEffect, useMemo, useRef, useState } from 'preact/hooks'
import BrandMark from './BrandMark'
import { Menu, X, ArrowUpRight, Printer } from './Icons'
import { routes } from '../content/routes'
import { cv } from '../content/cv'

function getHashPath() {
  const hash = window.location.hash || '#/'
  const raw = hash.startsWith('#') ? hash.slice(1) : hash
  return raw || '/'
}

export default function Header() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [activePath, setActivePath] = useState('/')
  const [scrolled, setScrolled] = useState(false)
  const navRef = useRef(null)

  useEffect(() => {
    setActivePath(getHashPath())
    const onHash = () => {
      setActivePath(getHashPath())
      setMobileMenuOpen(false)
    }
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  useEffect(() => {
    const nav = navRef.current
    if (!nav) return
    const reduced =
      window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches

    const update = () => {
      const overflowing = nav.scrollWidth > nav.clientWidth + 1
      nav.classList.toggle('is-overflowing', overflowing)
      nav.classList.remove('is-at-start', 'is-in-middle', 'is-at-end')
      if (!overflowing) return

      const max = nav.scrollWidth - nav.clientWidth
      const sl = nav.scrollLeft
      if (sl <= 1) nav.classList.add('is-at-start')
      else if (sl >= max - 1) nav.classList.add('is-at-end')
      else nav.classList.add('is-in-middle')
    }

    const ensureActiveVisible = () => {
      const active = nav.querySelector('.nav-link.is-active')
      if (!active) return

      const pad = 16
      const viewLeft = nav.scrollLeft
      const viewRight = viewLeft + nav.clientWidth
      const itemLeft = active.offsetLeft - pad
      const itemRight = active.offsetLeft + active.offsetWidth + pad

      let next = viewLeft
      if (itemLeft < viewLeft) next = itemLeft
      else if (itemRight > viewRight) next = itemRight - nav.clientWidth

      nav.scrollTo({ left: Math.max(0, next), behavior: reduced ? 'auto' : 'smooth' })
    }

    const onResize = () => update()
    const onScroll = () => update()
    window.addEventListener('resize', onResize, { passive: true })
    nav.addEventListener('scroll', onScroll, { passive: true })

    // Toggle overflow classes first, then scroll once layout updates.
    update()
    requestAnimationFrame(() => {
      ensureActiveVisible()
      update()
    })

    return () => {
      window.removeEventListener('resize', onResize)
      nav.removeEventListener('scroll', onScroll)
    }
  }, [activePath])

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 12)
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  useEffect(() => {
    if (!mobileMenuOpen) return
    const onKeyDown = (e) => {
      if (e.key === 'Escape') setMobileMenuOpen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [mobileMenuOpen])

  useEffect(() => {
    if (typeof document === 'undefined') return
    const prev = document.body.style.overflow
    if (mobileMenuOpen) document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = prev
    }
  }, [mobileMenuOpen])

  useEffect(() => {
    if (typeof document === 'undefined') return
    if (!mobileMenuOpen) return

    const el = document.createElement('button')
    el.type = 'button'
    el.className = 'nav-backdrop'
    el.setAttribute('aria-label', 'Close menu')
    el.addEventListener('click', () => setMobileMenuOpen(false))
    document.body.appendChild(el)

    return () => {
      el.remove()
    }
  }, [mobileMenuOpen])

  const navLinks = useMemo(() => routes.filter(r => r.href !== '/resume'), [])

  const handleNavClick = (e) => {
    const href = e.currentTarget.getAttribute('href')
    if (!href?.startsWith('/')) return
    e.preventDefault()
    e.stopPropagation()
    window.location.hash = href
    setMobileMenuOpen(false)
  }

  const skipToContent = () => {
    const el = document.getElementById('main')
    if (!el) return
    el.focus({ preventScroll: true })
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  return (
    <header className={`site-header ${scrolled ? 'is-scrolled' : ''}`}>
      <button className="skip-link" type="button" onClick={skipToContent}>
        Skip to content
      </button>

      <div className="container header-row">
        <a className="brand" href="/" onClick={handleNavClick} aria-label="Home">
          <BrandMark label="CV" subtitle="Early Years" />
          <div className="brand-text">
            <div className="brand-name">{cv.name}</div>
            <div className="brand-role">{cv.role}</div>
          </div>
        </a>

        <nav
          id="primary-nav"
          ref={navRef}
          className={`site-nav ${mobileMenuOpen ? 'is-open' : ''}`}
          aria-label="Primary"
        >
          {navLinks.map(link => {
            const isActive = activePath === link.href
            return (
              <a
                key={link.href}
                href={link.href}
                onClick={handleNavClick}
                className={`nav-link ${isActive ? 'is-active' : ''}`}
                aria-current={isActive ? 'page' : undefined}
              >
                <span className="nav-code">({link.code})</span>
                <span className="nav-label">{link.label}</span>
              </a>
            )
          })}
        </nav>

        <div className="header-actions">
          <a
            className={`btn btn-ghost header-resume ${activePath === '/resume' ? 'is-active' : ''}`}
            href="/resume"
            onClick={handleNavClick}
            aria-current={activePath === '/resume' ? 'page' : undefined}
            aria-label="Resume"
          >
            <Printer size={18} />
            <span>Resume</span>
          </a>
          <a
            className={`btn btn-primary header-contact ${activePath === '/contact' ? 'is-active' : ''}`}
            href="/contact"
            onClick={handleNavClick}
            aria-current={activePath === '/contact' ? 'page' : undefined}
            aria-label="Contact"
          >
            <ArrowUpRight size={18} />
            <span>Contact</span>
          </a>

          <button
            className="mobile-menu-btn"
            type="button"
            onClick={() => setMobileMenuOpen(v => !v)}
            aria-label={mobileMenuOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={mobileMenuOpen}
            aria-controls="primary-nav"
          >
            {mobileMenuOpen ? <X size={22} /> : <Menu size={22} />}
          </button>
        </div>
      </div>
    </header>
  )
}
