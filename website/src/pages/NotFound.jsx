import Page from '../components/Page'
import { ArrowRight } from '../components/Icons'

export default function NotFound() {
  const handleNavClick = (e) => {
    const href = e.currentTarget.getAttribute('href')
    if (!href?.startsWith('/')) return
    e.preventDefault()
    e.stopPropagation()
    window.location.hash = href
  }

  return (
    <Page kicker="(404)" title="Page not found" lead="This route doesn’t exist (yet).">
      <a className="btn btn-primary" href="/" onClick={handleNavClick}>
        <ArrowRight size={18} />
        <span>Back home</span>
      </a>
    </Page>
  )
}
