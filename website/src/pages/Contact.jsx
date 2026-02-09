import Page from '../components/Page'
import CopyButton from '../components/CopyButton'
import { ArrowUpRight, Mail, Phone, MapPin, Link as LinkIcon } from '../components/Icons'
import { cv } from '../content/cv'

export default function Contact() {
  const emailHref = `mailto:${cv.email}?subject=${encodeURIComponent('Early Years Teaching Opportunity')}`

  return (
    <Page
      kicker="(06_contact)"
      title="Contact"
      lead="If you’d like to talk about a preschool role, I reply quickly and can share references on request."
    >
      <div className="two-col">
        <div className="card">
          <div className="kicker">(direct)</div>

          <div className="contact-row">
            <div className="contact-icon">
              <Mail size={20} />
            </div>
            <div className="contact-main">
              <div className="contact-label">Email</div>
              <a className="contact-value" href={emailHref}>
                {cv.email}
              </a>
            </div>
            <CopyButton value={cv.email} label="Copy email" />
          </div>

          <div className="contact-row">
            <div className="contact-icon">
              <Phone size={20} />
            </div>
            <div className="contact-main">
              <div className="contact-label">Phone</div>
              <a className="contact-value" href={`tel:${cv.phone.replace(/[^+\\d]/g, '')}`}>
                {cv.phone}
              </a>
            </div>
            <CopyButton value={cv.phone} label="Copy phone" />
          </div>

          <div className="contact-row">
            <div className="contact-icon">
              <MapPin size={20} />
            </div>
            <div className="contact-main">
              <div className="contact-label">Location</div>
              <div className="contact-value">{cv.location}</div>
            </div>
          </div>

          <div className="divider" />

          <a className="btn btn-primary" href={emailHref}>
            <ArrowUpRight size={18} />
            <span>Email me</span>
          </a>
        </div>

        <div className="card-stack">
          <div className="card">
            <div className="kicker">(links)</div>
            <div className="link-list">
              {Object.entries(cv.links).map(([k, url]) => (
                <a key={k} className="ext-link" href={url} target="_blank" rel="noopener noreferrer">
                  <span className="ext-icon">
                    <LinkIcon size={18} />
                  </span>
                  <span className="ext-label">{k}</span>
                  <ArrowUpRight size={16} />
                </a>
              ))}
            </div>
          </div>

          <details className="card details-card">
            <summary className="details-summary">
              <span className="kicker">(note)</span>
              <span className="details-title">Customize this site</span>
            </summary>
            <p className="prose details-body">
              Update your details in <code>website/src/content/cv.js</code>, including links and Unsplash photos.
            </p>
          </details>
        </div>
      </div>
    </Page>
  )
}
