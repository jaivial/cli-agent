import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { routes } from '../data/routes';
import { homePageImages, galleryImages } from '../data/routes';
import { CountdownTimer } from '../components/CountdownTimer';
import { WeatherWidget } from '../components/WeatherWidget';
import { NewsletterSignup } from '../components/NewsletterSignup';

const pageVariants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.5 } },
};

export default function CarnivalHome() {
  const { language, toggleLanguage } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  const navItems = [
    { path: '/carnival', label: t('nav.home'), end: true },
    { path: '/carnival/sambodromo', label: t('nav.sambodromo') },
    { path: '/carnival/centro-lapa', label: t('nav.centrolapa') },
    { path: '/carnival/orla', label: t('nav.orla') },
    { path: '/carnival/schedule', label: t('nav.schedule') },
    { path: '/carnival/tips', label: t('nav.tips') },
  ];

  return (
    <motion.div
      data-carnival-home
      variants={pageVariants}
      initial="initial"
      animate="animate"
    >
      <motion.header
        className="carnival-topbar"
        initial={{ y: -100 }}
        animate={{ y: 0 }}
        transition={{ type: 'spring', stiffness: 100, damping: 20 }}
      >
        <div className="carnival-topbar-inner">
          <Link to="/carnival" className="carnival-brand">
            <span>🎭</span> Carnaval Rio
          </Link>
          <nav className="carnival-nav">
            {navItems.map((item) => (
              <Link
                key={item.path}
                to={item.path}
                className={item.end && window.location.pathname === item.path ? 'active' : ''}
              >
                {item.label}
              </Link>
            ))}
          </nav>
          <div className="carnival-topbar-actions">
            <div className="carnival-lang-toggle" role="group" aria-label="Idioma">
              <button
                className={language === 'pt' ? 'active' : ''}
                onClick={() => language !== 'pt' && toggleLanguage()}
                aria-pressed={language === 'pt'}
              >
                PT
              </button>
              <button
                className={language === 'es' ? 'active' : ''}
                onClick={() => language !== 'es' && toggleLanguage()}
                aria-pressed={language === 'es'}
              >
                ES
              </button>
            </div>
          </div>
        </div>
      </motion.header>

      <motion.section 
        className="home-hero" 
        variants={itemVariants}
        style={{ backgroundImage: `url(${homePageImages.heroMain})` }}
      >
        <div className="home-hero-overlay" />
        <div className="home-hero-content">
          <h1>{t('home.hero.title')}</h1>
          <p>{t('home.hero.subtitle')}</p>
        </div>
      </motion.section>

      <div className="home-widgets">
        <CountdownTimer />
        <WeatherWidget />
      </div>

      <motion.section className="home-highlights" variants={itemVariants}>
        <div className="highlights-grid">
          <motion.div 
            className="highlight-card left"
            variants={itemVariants}
            style={{ backgroundImage: `url(${homePageImages.heroLeft})` }}
          >
            <div className="highlight-overlay" />
            <h3>{t('home.highlights.parade')}</h3>
          </motion.div>
          <motion.div 
            className="highlight-card right"
            variants={itemVariants}
            style={{ backgroundImage: `url(${homePageImages.heroRight})` }}
          >
            <div className="highlight-overlay" />
            <h3>{t('home.highlights.street')}</h3>
          </motion.div>
        </div>
      </motion.section>

      <motion.section className="home-routes" variants={itemVariants}>
        <h2>{t('home.routes.title')}</h2>
        <div className="route-cards-grid">
          {routes.map((route, index) => (
            <motion.div
              key={route.id}
              className="route-card"
              variants={itemVariants}
              whileHover={{ scale: 1.03 }}
              style={{ borderColor: route.color }}
            >
              <div 
                className="route-card-image"
                style={{ backgroundImage: `url(${route.heroImage})` }}
              />
              <div className="route-card-content">
                <span className="route-card-type" style={{ color: route.color }}>
                  {t(route.subtitleKey)}
                </span>
                <h3>{t(route.titleKey)}</h3>
                <p>{t(route.descriptionKey)}</p>
                <Link 
                  to={`/carnival/${route.id === 'sambodromo' ? 'sambodromo' : route.id === 'centro' ? 'centro-lapa' : 'orla'}`}
                  className="route-card-btn"
                  style={{ background: route.color }}
                >
                  {t('home.routes.explore')}
                </Link>
              </div>
            </motion.div>
          ))}
        </div>
      </motion.section>

      <motion.section className="home-gallery" variants={itemVariants}>
        <h2>{t('home.gallery.title')}</h2>
        <div className="gallery-grid">
          {galleryImages.map((img, index) => (
            <motion.img
              key={index}
              src={img}
              alt={`Gallery ${index + 1}`}
              variants={itemVariants}
              whileHover={{ scale: 1.05 }}
            />
          ))}
        </div>
      </motion.section>

      <section className="home-newsletter">
        <NewsletterSignup />
      </section>

      <footer className="carnival-footer">
        <div className="carnival-footer-links">
          <span>{t('footer.contact')}</span>
          <span>{t('footer.follow')}</span>
        </div>
        <p>{t('footer.copyright')}</p>
      </footer>

      <style>{`
        .carnival-topbar {
          position: sticky;
          top: 0;
          z-index: 200;
          backdrop-filter: blur(16px);
          background: rgba(6, 13, 31, 0.85);
          border-bottom: 1px solid rgba(255, 255, 255, 0.08);
        }

        .carnival-topbar-inner {
          max-width: 1400px;
          margin: 0 auto;
          padding: 1rem 2rem;
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 1.5rem;
        }

        .carnival-brand {
          font-family: 'Fraunces', serif;
          font-size: 1.5rem;
          font-weight: 700;
          letter-spacing: 0.02em;
          color: #ffdf00;
          display: flex;
          align-items: center;
          gap: 0.5rem;
          text-decoration: none;
        }

        .carnival-nav {
          display: flex;
          gap: 1rem;
          font-size: 0.85rem;
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.08em;
        }

        .carnival-nav a {
          position: relative;
          padding: 0.5rem 0.8rem;
          border-radius: 1.5rem;
          transition: all 0.3s ease;
          color: rgba(255, 255, 255, 0.8);
          text-decoration: none;
        }

        .carnival-nav a.active,
        .carnival-nav a:hover {
          color: #ffdf00;
          background: rgba(255, 223, 0, 0.1);
        }

        .carnival-topbar-actions {
          display: flex;
          align-items: center;
          gap: 0.8rem;
        }

        .carnival-lang-toggle {
          display: flex;
          gap: 0.3rem;
          padding: 0.2rem;
          background: rgba(255, 255, 255, 0.08);
          border-radius: 999px;
        }

        .carnival-lang-toggle button {
          border: none;
          background: transparent;
          color: #f6f1e5;
          font-weight: 700;
          padding: 0.35rem 0.9rem;
          border-radius: 999px;
          cursor: pointer;
          font-size: 0.85rem;
          transition: all 0.3s ease;
        }

        .carnival-lang-toggle button.active {
          background: linear-gradient(135deg, #ff00ff, #ff6b3d);
          color: white;
        }

        .home-hero {
          position: relative;
          min-height: 60vh;
          display: flex;
          align-items: center;
          justify-content: center;
          background-size: cover;
          background-position: center;
          border-radius: 2rem;
          margin: 1rem;
          overflow: hidden;
        }

        .home-hero-overlay {
          position: absolute;
          inset: 0;
          background: linear-gradient(135deg, rgba(26, 5, 51, 0.9) 0%, rgba(45, 27, 78, 0.7) 50%, rgba(107, 45, 92, 0.5) 100%);
        }

        .home-hero-content {
          position: relative;
          z-index: 1;
          text-align: center;
          max-width: 800px;
          padding: 2rem;
        }

        .home-hero h1 {
          font-family: 'Fraunces', serif;
          font-size: clamp(3rem, 6vw, 5rem);
          color: #ffdf00;
          margin-bottom: 1rem;
        }

        .home-hero p {
          font-size: 1.3rem;
          color: rgba(255, 255, 255, 0.85);
        }

        .home-widgets {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
          gap: 1.5rem;
          margin-bottom: 2rem;
          padding: 0 1rem;
        }

        .home-highlights {
          padding: 2rem 1rem;
        }

        .highlights-grid {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 1.5rem;
          max-width: 1400px;
          margin: 0 auto;
        }

        .highlight-card {
          position: relative;
          min-height: 300px;
          border-radius: 2rem;
          background-size: cover;
          background-position: center;
          overflow: hidden;
        }

        .highlight-overlay {
          position: absolute;
          inset: 0;
          background: linear-gradient(to top, rgba(6, 15, 37, 0.95) 0%, transparent 60%);
        }

        .highlight-card h3 {
          position: absolute;
          bottom: 2rem;
          left: 2rem;
          font-family: 'Fraunces', serif;
          font-size: 1.8rem;
          color: #ffdf00;
          margin: 0;
        }

        .home-routes {
          padding: 2rem 1rem;
          max-width: 1400px;
          margin: 0 auto;
        }

        .home-routes h2 {
          font-family: 'Fraunces', serif;
          font-size: 2.5rem;
          color: #ffdf00;
          text-align: center;
          margin-bottom: 2rem;
        }

        .route-cards-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
          gap: 2rem;
        }

        .route-card {
          background: rgba(6, 15, 37, 0.75);
          border-radius: 2rem;
          border: 2px solid;
          overflow: hidden;
          transition: all 0.3s ease;
        }

        .route-card-image {
          height: 200px;
          background-size: cover;
          background-position: center;
        }

        .route-card-content {
          padding: 1.5rem;
        }

        .route-card-type {
          font-size: 0.85rem;
          text-transform: uppercase;
          letter-spacing: 0.15em;
          font-weight: 700;
        }

        .route-card h3 {
          font-family: 'Fraunces', serif;
          font-size: 1.5rem;
          color: white;
          margin: 0.5rem 0;
        }

        .route-card p {
          color: rgba(255, 255, 255, 0.7);
          margin-bottom: 1rem;
        }

        .route-card-btn {
          display: inline-block;
          padding: 0.8rem 1.5rem;
          border-radius: 2rem;
          color: #060f25;
          font-weight: 700;
          text-decoration: none;
          transition: all 0.3s ease;
        }

        .route-card-btn:hover {
          transform: translateY(-2px);
          box-shadow: 0 5px 20px rgba(0, 0, 0, 0.3);
        }

        .home-gallery {
          padding: 2rem 1rem;
          max-width: 1400px;
          margin: 0 auto;
        }

        .home-gallery h2 {
          font-family: 'Fraunces', serif;
          font-size: 2.5rem;
          color: #ffdf00;
          text-align: center;
          margin-bottom: 2rem;
        }

        .gallery-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
          gap: 1rem;
        }

        .gallery-grid img {
          width: 100%;
          height: 200px;
          object-fit: cover;
          border-radius: 2rem;
          cursor: pointer;
        }

        .home-newsletter {
          margin: 3rem auto;
          max-width: 600px;
          padding: 0 1rem;
        }

        .carnival-footer {
          border-top: 1px solid rgba(255, 255, 255, 0.08);
          padding: 2.5rem 0 3.5rem;
          text-align: center;
          color: rgba(255, 255, 255, 0.6);
        }

        .carnival-footer-links {
          display: flex;
          justify-content: center;
          flex-wrap: wrap;
          gap: 1.2rem;
          margin-bottom: 1rem;
        }

        @media (max-width: 768px) {
          .highlights-grid {
            grid-template-columns: 1fr;
          }
          
          .carnival-nav {
            display: none;
          }
        }
      `}</style>
    </motion.div>
  );
}
