import { Router } from 'preact-router'
import { useEffect } from 'preact/hooks'
import Header from './components/Header'
import Footer from './components/Footer'
import Home from './pages/Home'
import Profile from './pages/Profile'
import Experience from './pages/Experience'
import Education from './pages/Education'
import Portfolio from './pages/Portfolio'
import Philosophy from './pages/Philosophy'
import Contact from './pages/Contact'
import Resume from './pages/Resume'
import NotFound from './pages/NotFound'
import hashHistory from './router/hashHistory'
import { routes } from './content/routes'
import { cv } from './content/cv'

// Handle route changes for hash-based routing
const onChange = e => {
  const prefersReducedMotion =
    window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches

  const url = (e && typeof e.url === 'string' ? e.url : '').split('?')[0] || '/'
  const routeLabel = routes.find(r => r.href === url)?.label || (url === '/resume' ? 'Resume' : 'CV')
  document.title = `${routeLabel} — ${cv.name}`

  window.scrollTo({ top: 0, left: 0, behavior: prefersReducedMotion ? 'auto' : 'smooth' })
}

export default function App() {
  useEffect(() => {
    const url = hashHistory?.location?.pathname || '/'
    const routeLabel = routes.find(r => r.href === url)?.label || (url === '/resume' ? 'Resume' : 'CV')
    document.title = `${routeLabel} — ${cv.name}`
  }, [])

  return (
    <div className="app">
      <Header />
      <main id="main" className="main" tabIndex="-1">
        <Router history={hashHistory} onChange={onChange}>
          <Home path="/" />
          <Profile path="/profile" />
          <Experience path="/experience" />
          <Education path="/education" />
          <Portfolio path="/portfolio" />
          <Philosophy path="/philosophy" />
          <Contact path="/contact" />
          <Resume path="/resume" />
          <NotFound default />
        </Router>
      </main>
      <Footer />
    </div>
  )
}
