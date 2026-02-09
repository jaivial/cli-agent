import Page from '../components/Page'
import { cv } from '../content/cv'
import UnsplashImage from '../components/UnsplashImage'

export default function Profile() {
  return (
    <Page
      kicker="(01_profile)"
      title="Profile"
      lead="A quick, human overview of how I work with preschoolers (ages 3–4)."
    >
      <div className="two-col">
        <div>
          {cv.summary.map(p => (
            <p key={p} className="prose">
              {p}
            </p>
          ))}

          <div className="card">
            <div className="kicker">(values)</div>
            <div className="value-grid">
              <div className="value-card">
                <div className="value-title">Warm structure</div>
                <div className="value-body">Predictable routines + gentle boundaries.</div>
              </div>
              <div className="value-card">
                <div className="value-title">Play-based learning</div>
                <div className="value-body">Invitations to play that extend thinking and language.</div>
              </div>
              <div className="value-card">
                <div className="value-title">Family partnership</div>
                <div className="value-body">Clear communication, shared goals, mutual trust.</div>
              </div>
              <div className="value-card">
                <div className="value-title">Inclusion</div>
                <div className="value-body">Support diverse learners with thoughtful adaptations.</div>
              </div>
            </div>
          </div>
        </div>

        <div className="card-stack">
          <div className="card photo-card">
            <div className="surface-head">
              <div className="kicker">(classroom.vibe)</div>
              <div className="surface-dots" aria-hidden="true">
                <span className="dot dot-warn" />
                <span className="dot dot-ok" />
                <span className="dot dot-info" />
              </div>
            </div>
            <div className="photo-wrap photo-wrap-sm">
              <UnsplashImage
                id={cv.media?.profile?.id}
                alt={cv.media?.profile?.alt}
                className="photo-img"
                sizes="(max-width: 940px) 100vw, 520px"
              />
            </div>
            <div className="photo-caption">{cv.media?.profile?.caption}</div>
          </div>

          <div className="card">
            <div className="kicker">(core.skills)</div>
            <div className="chip-grid">
              {cv.coreSkills.map(s => (
                <span key={s} className="chip">
                  {s}
                </span>
              ))}
            </div>
          </div>

          <div className="card">
            <div className="kicker">(tools)</div>
            <ul className="list">
              {cv.tools.map(t => (
                <li key={t}>{t}</li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </Page>
  )
}
