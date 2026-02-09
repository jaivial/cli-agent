import Page from '../components/Page'
import { cv } from '../content/cv'
import UnsplashImage from '../components/UnsplashImage'

export default function Portfolio() {
  return (
    <Page
      kicker="(04_portfolio)"
      title="Portfolio"
      lead="A few classroom activities designed for ages 3–4 (swap in your own examples + photos)."
    >
      <div className="portfolio-grid">
        {cv.portfolio.map(item => (
          <article key={item.title} className="portfolio-card">
            {item.image?.id ? (
              <figure className="portfolio-figure">
                <UnsplashImage
                  id={item.image.id}
                  alt={item.image.alt}
                  className="portfolio-img"
                  sizes="(max-width: 940px) 100vw, 560px"
                />
                <figcaption className="portfolio-figcap">
                  <span className="portfolio-figlabel">(photo)</span>
                  <span className="portfolio-figvalue">Unsplash</span>
                </figcaption>
              </figure>
            ) : null}

            <div className="portfolio-head">
              <div className="kicker">(activity)</div>
              <h3 className="portfolio-title">{item.title}</h3>
              <div className="portfolio-focus">{item.focus}</div>
            </div>
            <p className="prose">{item.description}</p>
            <div className="chip-row">
              {item.tags.map(t => (
                <span key={t} className="chip chip-soft">
                  {t}
                </span>
              ))}
            </div>
          </article>
        ))}
      </div>

      <div className="card callout">
        <div className="kicker">(tip)</div>
        <p className="prose">
          Replace the text examples with your own work samples: weekly themes, learning stories, classroom
          displays, and photos of provocation setups (with family consent and school policy).
        </p>
      </div>
    </Page>
  )
}
