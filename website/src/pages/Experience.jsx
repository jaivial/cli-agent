import Page from '../components/Page'
import { cv } from '../content/cv'

export default function Experience() {
  return (
    <Page
      kicker="(02_experience)"
      title="Experience"
      lead="Roles, routines, and outcomes — written for fast scanning by a school leader."
    >
      <div className="timeline">
        {cv.experience.map(job => (
          <article key={`${job.title}-${job.org}-${job.start}`} className="timeline-item">
            <div className="timeline-left">
              <div className="timeline-role">{job.title}</div>
              <div className="timeline-org">{job.org}</div>
              <div className="timeline-loc">{job.location}</div>
              <div className="timeline-dates">
                {job.start} — {job.end}
              </div>
            </div>
            <div className="timeline-right">
              <ul className="list">
                {job.bullets.map(b => (
                  <li key={b}>{b}</li>
                ))}
              </ul>
            </div>
          </article>
        ))}
      </div>
    </Page>
  )
}

