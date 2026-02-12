import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { heroImages } from '../data/gallery';

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.15,
      delayChildren: 0.3,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 30 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.6, ease: 'easeOut' },
  },
};

export function Hero() {
  const { language } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  return (
    <section data-hero-section className="carnival-hero">
      <div className="carnival-hero-content">
        <motion.div
          variants={containerVariants}
          initial="hidden"
          animate="visible"
        >
          <motion.p
            variants={itemVariants}
            className="carnival-eyebrow"
          >
            {t('hero.eyebrow')}
          </motion.p>
          <motion.h1 variants={itemVariants}>
            {t('hero.title')}
          </motion.h1>
          <motion.p variants={itemVariants} className="carnival-lead">
            {t('hero.lead')}
          </motion.p>
          <motion.div variants={itemVariants} className="carnival-hero-ctas">
            <Link to="/carnival/sambodromo" className="cta-primary">
              {t('hero.cta.explore')}
            </Link>
            <Link to="/carnival/schedule" className="cta-secondary">
              {t('hero.cta.schedule')}
            </Link>
          </motion.div>
          <motion.div variants={itemVariants} className="carnival-hero-stats">
            <motion.div
              className="carnival-stat"
              whileHover={{ scale: 1.05 }}
              transition={{ type: 'spring', stiffness: 300 }}
            >
              <strong>500+</strong>
              <span>{t('hero.stat.blocks')}</span>
            </motion.div>
            <motion.div
              className="carnival-stat"
              whileHover={{ scale: 1.05 }}
              transition={{ type: 'spring', stiffness: 300 }}
            >
              <strong>3</strong>
              <span>{t('hero.stat.nights')}</span>
            </motion.div>
            <motion.div
              className="carnival-stat"
              whileHover={{ scale: 1.05 }}
              transition={{ type: 'spring', stiffness: 300 }}
            >
              <strong>24h</strong>
              <span>{t('hero.stat.rhythm')}</span>
            </motion.div>
          </motion.div>
        </motion.div>
      </div>

      <div className="carnival-hero-media">
        <motion.figure
          className="carnival-hero-photo main"
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.8, delay: 0.2 }}
        >
          <img src={heroImages.main} alt="Desfiles no Sambódromo" />
          <div className="photo-gradient" />
          <figcaption>{t('media.desfiles')}</figcaption>
        </motion.figure>
        <motion.figure
          className="carnival-hero-photo left"
          initial={{ opacity: 0, x: -50 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.6, delay: 0.4 }}
        >
          <img src={heroImages.left} alt="Samba e bateria" />
          <div className="photo-gradient" />
          <figcaption>{t('media.samba')}</figcaption>
        </motion.figure>
        <motion.figure
          className="carnival-hero-photo right"
          initial={{ opacity: 0, x: 50 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.6, delay: 0.5 }}
        >
          <img src={heroImages.right} alt="Máscaras e brilho" />
          <div className="photo-gradient" />
          <figcaption>{t('media.masks')}</figcaption>
        </motion.figure>
      </div>

      <style>{`
        .carnival-hero {
          min-height: 85vh;
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
          gap: 3rem;
          align-items: center;
          padding-top: 2rem;
        }

        .carnival-hero-content {
          max-width: 600px;
        }

        .carnival-eyebrow {
          text-transform: uppercase;
          letter-spacing: 0.28em;
          font-size: 0.8rem;
          color: rgba(255, 255, 255, 0.6);
          margin-bottom: 1rem;
        }

        .carnival-hero h1 {
          font-family: 'Fraunces', serif;
          font-size: clamp(2.8rem, 5vw, 4.5rem);
          line-height: 1.05;
          background: linear-gradient(135deg, #ffdf00 0%, #ff6b3d 50%, #ff00ff 100%);
          -webkit-background-clip: text;
          -webkit-text-fill-color: transparent;
          background-clip: text;
        }

        .carnival-lead {
          font-size: 1.2rem;
          margin: 1.5rem 0 2rem;
          color: rgba(255, 255, 255, 0.86);
          line-height: 1.6;
        }

        .carnival-hero-ctas {
          display: flex;
          flex-wrap: wrap;
          gap: 1rem;
        }

        .cta-primary,
        .cta-secondary {
          padding: 1rem 2rem;
          border-radius: 2rem;
          font-weight: 700;
          font-size: 1rem;
          letter-spacing: 0.02em;
          transition: transform 0.3s ease, box-shadow 0.3s ease;
          display: inline-flex;
          align-items: center;
          gap: 0.5rem;
          text-decoration: none;
        }

        .cta-primary {
          background: linear-gradient(135deg, #ffdf00, #ff6b3d);
          color: #060f25;
          box-shadow: 0 15px 35px rgba(255, 223, 0, 0.25);
        }

        .cta-secondary {
          border: 2px solid rgba(255, 255, 255, 0.3);
          color: #f6f1e5;
          backdrop-filter: blur(10px);
        }

        .cta-primary:hover,
        .cta-secondary:hover {
          transform: translateY(-3px);
        }

        .cta-primary:hover {
          box-shadow: 0 20px 40px rgba(255, 223, 0, 0.35);
        }

        .carnival-hero-stats {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
          gap: 1rem;
          margin-top: 2.5rem;
        }

        .carnival-stat {
          padding: 1.2rem;
          border-radius: 2rem;
          background: rgba(6, 15, 37, 0.7);
          border: 1px solid rgba(255, 223, 0, 0.2);
          text-align: center;
        }

        .carnival-stat strong {
          display: block;
          font-family: 'Fraunces', serif;
          font-size: 1.5rem;
          color: #ffdf00;
        }

        .carnival-stat span {
          font-size: 0.85rem;
          color: rgba(255, 255, 255, 0.7);
        }

        .carnival-hero-media {
          display: grid;
          grid-template-columns: repeat(6, 1fr);
          grid-auto-rows: 80px;
          gap: 1rem;
        }

        .carnival-hero-photo {
          position: relative;
          border-radius: 2rem;
          overflow: hidden;
          box-shadow: 0 25px 50px rgba(0, 0, 0, 0.4);
        }

        .carnival-hero-photo img {
          width: 100%;
          height: 100%;
          object-fit: cover;
        }

        .photo-gradient {
          position: absolute;
          inset: 0;
          background: linear-gradient(to top, rgba(6, 15, 37, 0.6) 0%, transparent 50%);
          pointer-events: none;
        }

        .carnival-hero-photo.main {
          grid-column: 1 / -1;
          grid-row: 1 / 4;
        }

        .carnival-hero-photo.left {
          grid-column: 1 / 4;
          grid-row: 4 / 6;
        }

        .carnival-hero-photo.right {
          grid-column: 4 / 7;
          grid-row: 4 / 6;
        }

        .carnival-hero-photo figcaption {
          position: absolute;
          bottom: 0.8rem;
          left: 0.8rem;
          background: rgba(6, 15, 37, 0.8);
          padding: 0.4rem 0.9rem;
          border-radius: 999px;
          font-size: 0.8rem;
          letter-spacing: 0.04em;
          font-weight: 600;
        }

        @media (max-width: 900px) {
          .carnival-hero {
            padding-top: 2rem;
          }

          .carnival-hero-media {
            grid-template-columns: repeat(2, 1fr);
            grid-auto-rows: 100px;
          }

          .carnival-hero-photo.main {
            grid-column: 1 / -1;
            grid-row: 1 / 3;
          }

          .carnival-hero-photo.left,
          .carnival-hero-photo.right {
            grid-column: auto;
            grid-row: auto;
          }
        }
      `}</style>
    </section>
  );
}
