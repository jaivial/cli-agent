import { cv } from '../content/cv'
import { routes } from '../content/routes'
import { ArrowUpRight, ArrowRight } from '../components/Icons'
import { useHeroEntrance, useStaggerChildren } from '../hooks/useGSAPAnimations'
import UnsplashImage from '../components/UnsplashImage'

export default function Home() {
  const heroRef = useHeroEntrance()
  const highlightsRef = useStaggerChildren(0.08, 0.15)

  const handleNavClick = (e) => {
    const href = e.currentTarget.getAttribute('href')
    if (!href?.startsWith('/')) return
    e.preventDefault()
    e.stopPropagation()
    window.location.hash = href
  }

  return (
    <div className="home">
      <section className="hero">
        <div className="container hero-grid" ref={heroRef}>
          <div className="hero-left">
            <div className="kicker hero-animate">(00_home.snapshot)</div>
            <h1 className="hero-title hero-animate">{cv.role}</h1>
            <p className="hero-lead hero-animate">{cv.headline}</p>

            <div className="hero-actions hero-animate">
              <a className="btn btn-primary" href="/contact" onClick={handleNavClick}>
                <ArrowUpRight size={18} />
                <span>Get in touch</span>
              </a>
              <a className="btn btn-ghost" href="/resume" onClick={handleNavClick}>
                <ArrowRight size={18} />
                <span>View resume</span>
              </a>
            </div>

            <div className="hero-meta hero-animate">
              <div className="meta-row">
                <span className="meta-label">(name)</span>
                <span className="meta-value">{cv.name}</span>
              </div>
              <div className="meta-row">
                <span className="meta-label">(base)</span>
                <span className="meta-value">{cv.location}</span>
              </div>
              <div className="meta-row">
                <span className="meta-label">(focus)</span>
                <span className="meta-value">Ages 3–4 • play • SEL • routines</span>
              </div>
            </div>
          </div>

          <div className="hero-right hero-animate">
            <div className="card photo-card">
              <div className="surface-head">
                <div className="kicker">(field.photo)</div>
                <div className="surface-dots" aria-hidden="true">
                  <span className="dot dot-warn" />
                  <span className="dot dot-ok" />
                  <span className="dot dot-info" />
                </div>
              </div>
              <div className="photo-wrap">
                <UnsplashImage
                  id={cv.media?.hero?.id}
                  alt={cv.media?.hero?.alt}
                  className="photo-img"
                  priority
                  sizes="(max-width: 980px) 100vw, 520px"
                />
              </div>
              <div className="photo-caption">{cv.media?.hero?.caption}</div>
            </div>

            <div className="panel panel-soft terminal-panel">
              <div className="panel-header">
                <div className="kicker">(quick.links)</div>
              </div>
              <div className="panel-body">
                <div className="quick-links">
                  {routes.filter(r => r.href !== '/').slice(0, 5).map(r => (
                    <a
                      key={r.href}
                      className="quick-link"
                      href={r.href}
                      onClick={handleNavClick}
                    >
                      <span className="quick-code">({r.code})</span>
                      <span className="quick-label">{r.label}</span>
                      <ArrowUpRight size={16} />
                    </a>
                  ))}
                </div>
              </div>
            </div>

            <div className="panel terminal-panel">
              <div className="panel-header">
                <div className="kicker">(highlights)</div>
              </div>
              <div className="panel-body" ref={highlightsRef}>
                {cv.highlights.map(h => (
                  <div key={h.label} className="stat-card">
                    <div className="stat-label">{h.label}</div>
                    <div className="stat-value">{h.value}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container two-col">
          <div>
            <div className="kicker">(01_profile.summary)</div>
            {cv.summary.map(p => (
              <p key={p} className="prose">
                {p}
              </p>
            ))}
          </div>
          <div className="card-stack">
            <div className="card">
              <div className="kicker">(core.skills)</div>
              <div className="chip-grid">
                {cv.coreSkills.slice(0, 8).map(s => (
                  <span key={s} className="chip">{s}</span>
                ))}
              </div>
            </div>
            <div className="card">
              <div className="kicker">(recent.work)</div>
              <div className="mini-list">
                {cv.portfolio.slice(0, 2).map(item => (
                  <div key={item.title} className="mini-item">
                    <div className="mini-thumb" aria-hidden="true">
                      {item.image?.id ? (
                        <UnsplashImage
                          id={item.image.id}
                          alt=""
                          className="mini-thumb-img"
                          sizes="88px"
                        />
                      ) : null}
                    </div>
                    <div className="mini-text">
                      <div className="mini-title">{item.title}</div>
                      <div className="mini-sub">{item.focus}</div>
                    </div>
                  </div>
                ))}
                <a className="mini-cta" href="/portfolio" onClick={handleNavClick}>
                  <span>See full portfolio</span>
                  <ArrowRight size={16} />
                </a>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}
