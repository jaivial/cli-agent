import { cv } from '../content/cv'
import { Printer } from '../components/Icons'

export default function Resume() {
  const onPrint = () => window.print()

  return (
    <div className="resume">
      <div className="container">
        <div className="resume-actions no-print">
          <button className="btn btn-ghost" type="button" onClick={onPrint}>
            <Printer size={18} />
            <span>Print / Save PDF</span>
          </button>
        </div>

        <div className="resume-sheet">
          <header className="resume-header">
            <div>
              <div className="kicker">(07_resume)</div>
              <h1 className="resume-name">{cv.name}</h1>
              <div className="resume-role">{cv.role}</div>
            </div>
            <div className="resume-contact">
              <a href={`mailto:${cv.email}`}>{cv.email}</a>
              <a href={`tel:${cv.phone.replace(/[^+\\d]/g, '')}`}>{cv.phone}</a>
              <div>{cv.location}</div>
            </div>
          </header>

          <section className="resume-section">
            <h2 className="resume-h">Summary</h2>
            {cv.summary.map(p => (
              <p key={p} className="resume-p">
                {p}
              </p>
            ))}
          </section>

          <section className="resume-section">
            <h2 className="resume-h">Experience</h2>
            <div className="resume-items">
              {cv.experience.map(job => (
                <div key={`${job.title}-${job.org}-${job.start}`} className="resume-item">
                  <div className="resume-item-head">
                    <div className="resume-item-title">
                      {job.title} — {job.org}
                    </div>
                    <div className="resume-item-meta">
                      <span>{job.location}</span>
                      <span>•</span>
                      <span>
                        {job.start}–{job.end}
                      </span>
                    </div>
                  </div>
                  <ul className="resume-list">
                    {job.bullets.map(b => (
                      <li key={b}>{b}</li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </section>

          <section className="resume-section resume-grid">
            <div>
              <h2 className="resume-h">Education</h2>
              <div className="resume-items">
                {cv.education.map(ed => (
                  <div key={`${ed.title}-${ed.org}-${ed.year}`} className="resume-item">
                    <div className="resume-item-title">{ed.title}</div>
                    <div className="resume-item-meta">
                      <span>{ed.org}</span>
                      <span>•</span>
                      <span>{ed.year}</span>
                    </div>
                    {ed.details?.length ? (
                      <div className="resume-tags">
                        {ed.details.map(d => (
                          <span key={d} className="resume-tag">
                            {d}
                          </span>
                        ))}
                      </div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>

            <div>
              <h2 className="resume-h">Skills</h2>
              <div className="resume-tags">
                {cv.coreSkills.map(s => (
                  <span key={s} className="resume-tag">
                    {s}
                  </span>
                ))}
              </div>

              <h2 className="resume-h resume-h-spaced">Certifications</h2>
              <div className="resume-items">
                {cv.certifications.map(c => (
                  <div key={`${c.title}-${c.year}`} className="resume-item">
                    <div className="resume-item-title">{c.title}</div>
                    <div className="resume-item-meta">
                      <span>{c.issuer}</span>
                      <span>•</span>
                      <span>{c.year}</span>
                    </div>
                  </div>
                ))}
              </div>

              <div className="resume-ref">{cv.references}</div>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

