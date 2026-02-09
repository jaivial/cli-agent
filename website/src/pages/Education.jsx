import Page from '../components/Page'
import { cv } from '../content/cv'

export default function Education() {
  return (
    <Page
      kicker="(03_education)"
      title="Education & Certifications"
      lead="Training, credentials, and safety essentials for early-years classrooms."
    >
      <div className="two-col">
        <div className="card">
          <div className="kicker">(education)</div>
          <div className="edu-list">
            {cv.education.map(ed => (
              <div key={`${ed.title}-${ed.org}-${ed.year}`} className="edu-item">
                <div className="edu-title">{ed.title}</div>
                <div className="edu-org">{ed.org}</div>
                <div className="edu-meta">
                  <span>{ed.location}</span>
                  <span>•</span>
                  <span>{ed.year}</span>
                </div>
                {ed.details?.length ? (
                  <div className="chip-grid">
                    {ed.details.map(d => (
                      <span key={d} className="chip chip-soft">
                        {d}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </div>

        <div className="card">
          <div className="kicker">(certifications)</div>
          <div className="cert-list">
            {cv.certifications.map(c => (
              <div key={`${c.title}-${c.year}`} className="cert-item">
                <div className="cert-title">{c.title}</div>
                <div className="cert-meta">
                  <span>{c.issuer}</span>
                  <span>•</span>
                  <span>{c.year}</span>
                </div>
              </div>
            ))}
          </div>

          <div className="divider" />

          <div className="kicker">(references)</div>
          <p className="prose">{cv.references}</p>
        </div>
      </div>
    </Page>
  )
}

