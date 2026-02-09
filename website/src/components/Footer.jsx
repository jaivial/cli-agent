import BrandMark from './BrandMark'
import { ArrowUpRight, Mail, Phone } from './Icons'
import { routes } from '../content/routes'
import { cv } from '../content/cv'

export default function Footer() {
  const handleNavClick = (e) => {
    const href = e.currentTarget.getAttribute('href')
    if (!href?.startsWith('/')) return
    e.preventDefault()
    e.stopPropagation()
    window.location.hash = href
  }

  return (
    <footer className="site-footer">
      <div className="container footer-grid">
        <div className="footer-brand">
          <a className="footer-home" href="/" onClick={handleNavClick}>
            <BrandMark label="CV" subtitle="Early Years" />
          </a>
          <div className="footer-title">{cv.name}</div>
          <div className="footer-subtitle">{cv.role}</div>
          <p className="footer-note">
            A calm, structured portfolio and resume site for early-years teaching.
          </p>
        </div>

        <div className="footer-links" aria-label="Footer links">
          {routes.slice(0, 5).map(r => (
            <a key={r.href} className="footer-link" href={r.href} onClick={handleNavClick}>
              <span>({r.code})</span>
              <span>{r.label}</span>
              <ArrowUpRight size={16} />
            </a>
          ))}
          <a className="footer-link" href="/resume" onClick={handleNavClick}>
            <span>(07_resume)</span>
            <span>Resume</span>
            <ArrowUpRight size={16} />
          </a>
        </div>

        <div className="footer-contact">
          <div className="kicker">(contact)</div>
          <a className="footer-contact-row" href={`mailto:${cv.email}`}>
            <Mail size={18} />
            <span>{cv.email}</span>
          </a>
          <a className="footer-contact-row" href={`tel:${cv.phone.replace(/[^+\\d]/g, '')}`}>
            <Phone size={18} />
            <span>{cv.phone}</span>
          </a>
          <div className="footer-location">{cv.location}</div>
        </div>
      </div>

      <div className="container footer-bottom">
        <div>© {new Date().getFullYear()} {cv.name}</div>
        <div className="footer-small">Editorial grid inspired by adoratorio.studio, nicolaromei.com, terminal-industries.com. Photos: Unsplash.</div>
      </div>
    </footer>
  )
}
