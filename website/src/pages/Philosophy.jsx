import Page from '../components/Page'
import { cv } from '../content/cv'

export default function Philosophy() {
  return (
    <Page
      kicker="(05_philosophy)"
      title="Teaching Philosophy"
      lead="What I believe children need at 3–4 years old — and what that looks like day to day."
    >
      <div className="philosophy-grid">
        {cv.philosophy.map(p => (
          <article key={p.title} className="card philosophy-card">
            <div className="kicker">(principle)</div>
            <h3 className="h3">{p.title}</h3>
            <p className="prose">{p.body}</p>
          </article>
        ))}
      </div>

      <div className="two-col">
        <div className="card">
          <div className="kicker">(classroom.signals)</div>
          <ul className="list">
            <li>Clear visual routines and predictable transitions.</li>
            <li>Invitations to play that encourage inquiry and language.</li>
            <li>Positive guidance, co-regulation, and repair after conflict.</li>
            <li>Small-group work that differentiates without labeling.</li>
          </ul>
        </div>
        <div className="card">
          <div className="kicker">(family.partnership)</div>
          <ul className="list">
            <li>Warm daily communication + consistent boundaries.</li>
            <li>Documented learning stories that make progress visible.</li>
            <li>Shared goals for independence, confidence, and kindness.</li>
          </ul>
        </div>
      </div>
    </Page>
  )
}

