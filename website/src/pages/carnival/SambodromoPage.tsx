import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { routes } from '../data/routes';
import { FloatingParticles } from '../components/FloatingParticles';
import { CountdownTimer } from '../components/CountdownTimer';
import { WeatherWidget } from '../components/WeatherWidget';
import { BookmarkButton } from '../components/BookmarkButton';
import { ShareButtons } from '../components/ShareButtons';
import { InteractiveMap } from '../components/InteractiveMap';

const route = routes[0];

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.1 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.5 } },
};

export default function SambodromoPage() {
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
      data-sambodromo-page
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <FloatingParticles count={25} />
      
      <div className="carnival-bg">
        <div className="carnival-bg-gradient" />
        <div className="glowing-orb orb-gold" />
        <div className="glowing-orb orb-orange" />
        <div className="glowing-orb orb-magenta" />
      </div>

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
              <Link key={item.path} to={item.path}>
                {item.label}
              </Link>
            ))}
          </nav>
          <div className="carnival-topbar-actions">
            <div className="carnival-lang-toggle" role="group" aria-label="Idioma">
              <button
                className={language === 'pt' ? 'active' : ''}
                onClick={() => language !== 'pt' && toggleLanguage()}
              >
                PT
              </button>
              <button
                className={language === 'es' ? 'active' : ''}
                onClick={() => language !== 'es' && toggleLanguage()}
              >
                ES
              </button>
            </div>
          </div>
        </div>
      </motion.header>

      <div className="route-page">
        <motion.div 
          className="route-hero" 
          variants={itemVariants}
          style={{ backgroundImage: `url(${route.heroImage})` }}
        >
          <div className="route-hero-overlay" />
          <div className="route-hero-content">
            <span className="route-type" style={{ color: route.color }}>
              {t(route.subtitleKey)}
            </span>
            <h1>{t(route.titleKey)}</h1>
            <p>{t(route.descriptionKey)}</p>
            <div className="route-hero-actions">
              <BookmarkButton routeId={route.id} />
              <ShareButtons title={t(route.titleKey)} />
            </div>
          </div>
        </motion.div>

        <div className="route-widgets">
          <CountdownTimer />
          <WeatherWidget />
        </div>

        <motion.section className="route-section" variants={itemVariants}>
          <h2>{t('routePage.about')}</h2>
          <p>{t('routes.sambodromo.desc')}</p>
          <div className="route-stops">
            <h3>{t('routes.itinerary')}</h3>
            <div className="stops-grid">
              {route.stops.map((stop, index) => (
                <motion.div
                  key={stop.name}
                  className="stop-card"
                  initial={{ opacity: 0, x: -20 }}
                  whileInView={{ opacity: 1, x: 0 }}
                  viewport={{ once: true }}
                  transition={{ delay: index * 0.1 }}
                >
                  <div 
                    className="stop-number"
                    style={{ background: route.color, boxShadow: `0 0 15px ${route.color}` }}
                  >
                    {stop.order}
                  </div>
                  <span>{t(stop.name)}</span>
                </motion.div>
              ))}
            </div>
          </div>
        </motion.section>

        <motion.section className="route-section" variants={itemVariants}>
          <h2>{t('routePage.itinerary')}</h2>
          <div className="itinerary-timeline">
            {route.itinerary.map((item, index) => (
              <motion.div
                key={item.time}
                className="itinerary-item"
                initial={{ opacity: 0, x: -20 }}
                whileInView={{ opacity: 1, x: 0 }}
                viewport={{ once: true }}
                transition={{ delay: index * 0.1 }}
              >
                <div className="itinerary-time" style={{ color: route.color }}>
                  {item.time}
                </div>
                <div className="itinerary-content">
                  <h4>{t(item.titleKey)}</h4>
                  <p>{t(item.descriptionKey)}</p>
                </div>
              </motion.div>
            ))}
          </div>
        </motion.section>

        <motion.section className="route-section" variants={itemVariants}>
          <h2>{t('routePage.map')}</h2>
          <InteractiveMap 
            markers={[
              { position: [-22.9118, -43.2093], title: 'Sambódromo', color: route.color },
              { position: [-22.9070, -43.1888], title: 'Centro', color: '#00b36b' },
            ]}
          />
        </motion.section>

        <motion.section className="route-section" variants={itemVariants}>
          <h2>{t('routePage.tips')}</h2>
          <div className="tips-grid">
            {route.tips.map((tipKey, index) => (
              <motion.div
                key={tipKey}
                className="tip-item"
                initial={{ opacity: 0, scale: 0.9 }}
                whileInView={{ opacity: 1, scale: 1 }}
                viewport={{ once: true }}
                transition={{ delay: index * 0.1 }}
              >
                <span className="tip-icon">💡</span>
                <span>{t(tipKey)}</span>
              </motion.div>
            ))}
          </div>
        </motion.section>

        <motion.section className="route-section" variants={itemVariants}>
          <h2>{t('routePage.gallery')}</h2>
          <div className="route-gallery">
            {route.gallery.map((img, index) => (
              <motion.img
                key={index}
                src={img}
                alt={`${t(route.titleKey)} ${index + 1}`}
                initial={{ opacity: 0, scale: 0.9 }}
                whileInView={{ opacity: 1, scale: 1 }}
                viewport={{ once: true }}
                transition={{ delay: index * 0.1 }}
                whileHover={{ scale: 1.05 }}
              />
            ))}
          </div>
        </motion.section>

        <motion.section className="route-nav" variants={itemVariants}>
          <Link to="/carnival/centro-lapa" className="route-nav-btn next">
            <span>{t('routePage.next')}: Centro + Lapa</span>
            <span className="nav-arrow">→</span>
          </Link>
        </motion.section>
      </div>

      <footer className="carnival-footer">
        <div className="carnival-footer-links">
          <span>{t('footer.contact')}</span>
          <span>{t('footer.follow')}</span>
        </div>
        <p>{t('footer.copyright')}</p>
      </footer>

      <style>{`
        .carnival-bg {
          position: fixed;
          inset: 0;
          z-index: -2;
          pointer-events: none;
        }

        .carnival-bg-gradient {
          position: absolute;
          inset: 0;
          background: linear-gradient(135deg, #1a0533 0%, #2d1b4e 25%, #4a1942 50%, #6b2d5c 75%, #ff6b3d 100%);
        }

        .glowing-orb {
          position: absolute;
          border-radius: 50%;
          filter: blur(40px);
          animation: orbPulse 6s ease-in-out infinite;
        }

        .orb-gold {
          width: 300px;
          height: 300px;
          top: 10%;
          left: 5%;
          background: radial-gradient(circle, rgba(255, 215, 0, 0.4) 0%, rgba(255, 215, 0, 0.1) 40%, transparent 70%);
        }

        .orb-orange {
          width: 250px;
          height: 250px;
          top: 60%;
          right: 10%;
          background: radial-gradient(circle, rgba(255, 107, 61, 0.4) 0%, rgba(255, 107, 61, 0.1) 40%, transparent 70%);
          animation-delay: 2s;
        }

        .orb-magenta {
          width: 200px;
          height: 200px;
          bottom: 20%;
          left: 20%;
          background: radial-gradient(circle, rgba(255, 0, 128, 0.35) 0%, rgba(255, 0, 128, 0.1) 40%, transparent 70%);
          animation-delay: 4s;
        }

        @keyframes orbPulse {
          0%, 100% { opacity: 0.5; transform: scale(1); }
          50% { opacity: 0.8; transform: scale(1.15); }
        }

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

        .route-page {
          padding-bottom: 4rem;
          max-width: 1400px;
          margin: 0 auto;
          padding-left: 2rem;
          padding-right: 2rem;
        }

        .route-hero {
          position: relative;
          min-height: 60vh;
          display: flex;
          align-items: flex-end;
          padding: 3rem;
          border-radius: 2rem;
          background-size: cover;
          background-position: center;
          margin-bottom: 2rem;
          margin-top: 1rem;
          overflow: hidden;
        }

        .route-hero-overlay {
          position: absolute;
          inset: 0;
          background: linear-gradient(to top, rgba(6, 15, 37, 0.95) 0%, rgba(6, 15, 37, 0.4) 50%, transparent 100%);
        }

        .route-hero-content {
          position: relative;
          z-index: 1;
          max-width: 700px;
        }

        .route-type {
          font-size: 0.85rem;
          text-transform: uppercase;
          letter-spacing: 0.15em;
          font-weight: 700;
        }

        .route-hero h1 {
          font-family: 'Fraunces', serif;
          font-size: clamp(2.5rem, 5vw, 4rem);
          color: #ffdf00;
          margin: 0.5rem 0;
        }

        .route-hero p {
          font-size: 1.2rem;
          color: rgba(255, 255, 255, 0.8);
          margin-bottom: 1.5rem;
        }

        .route-hero-actions {
          display: flex;
          flex-wrap: wrap;
          gap: 1rem;
          align-items: center;
        }

        .route-widgets {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
          gap: 1.5rem;
          margin-bottom: 3rem;
        }

        .route-section {
          margin-bottom: 3rem;
        }

        .route-section h2 {
          font-family: 'Fraunces', serif;
          font-size: 2rem;
          color: #ffdf00;
          margin-bottom: 1.5rem;
        }

        .route-section > p {
          color: rgba(255, 255, 255, 0.8);
          line-height: 1.7;
          max-width: 700px;
        }

        .route-stops {
          margin-top: 2rem;
        }

        .route-stops h3 {
          font-size: 1.1rem;
          color: rgba(255, 255, 255, 0.7);
          margin-bottom: 1rem;
        }

        .stops-grid {
          display: flex;
          flex-wrap: wrap;
          gap: 1rem;
        }

        .stop-card {
          display: flex;
          align-items: center;
          gap: 0.8rem;
          padding: 0.8rem 1.2rem;
          background: rgba(6, 15, 37, 0.75);
          border-radius: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .stop-number {
          width: 28px;
          height: 28px;
          border-radius: 50%;
          display: flex;
          align-items: center;
          justify-content: center;
          font-weight: 700;
          font-size: 0.85rem;
          color: #060f25;
        }

        .itinerary-timeline {
          display: flex;
          flex-direction: column;
          gap: 1.2rem;
        }

        .itinerary-item {
          display: flex;
          gap: 1.5rem;
          padding: 1.2rem;
          background: rgba(6, 15, 37, 0.6);
          border-radius: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.08);
        }

        .itinerary-time {
          font-family: 'Fraunces', serif;
          font-size: 1.3rem;
          font-weight: 700;
          min-width: 70px;
        }

        .itinerary-content h4 {
          font-size: 1.1rem;
          color: #ffdf00;
          margin-bottom: 0.3rem;
        }

        .itinerary-content p {
          color: rgba(255, 255, 255, 0.7);
          font-size: 0.95rem;
          margin: 0;
        }

        .tips-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
          gap: 1rem;
        }

        .tip-item {
          display: flex;
          align-items: center;
          gap: 0.8rem;
          padding: 1rem 1.2rem;
          background: rgba(6, 15, 37, 0.6);
          border-radius: 2rem;
          border: 1px solid rgba(255, 223, 0, 0.2);
        }

        .tip-icon {
          font-size: 1.3rem;
        }

        .route-gallery {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
          gap: 1rem;
        }

        .route-gallery img {
          width: 100%;
          height: 200px;
          object-fit: cover;
          border-radius: 2rem;
          cursor: pointer;
        }

        .route-nav {
          margin-top: 3rem;
          display: flex;
          justify-content: flex-end;
        }

        .route-nav-btn {
          display: flex;
          align-items: center;
          gap: 1rem;
          padding: 1rem 1.5rem;
          background: rgba(255, 223, 0, 0.15);
          border: 1px solid rgba(255, 223, 0, 0.4);
          border-radius: 2rem;
          color: #ffdf00;
          text-decoration: none;
          font-weight: 600;
          transition: all 0.3s ease;
        }

        .route-nav-btn:hover {
          background: rgba(255, 223, 0, 0.25);
          transform: translateX(-5px);
        }

        .nav-arrow {
          font-size: 1.3rem;
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
          .route-hero {
            min-height: 50vh;
            padding: 2rem;
          }

          .route-hero-actions {
            flex-direction: column;
            align-items: flex-start;
          }

          .itinerary-item {
            flex-direction: column;
            gap: 0.5rem;
          }
          
          .carnival-nav {
            display: none;
          }
        }
      `}</style>
    </motion.div>
  );
}
