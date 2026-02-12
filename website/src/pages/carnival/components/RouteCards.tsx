import { useState } from 'react';
import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { routes } from '../data/routes';
import { RouteMap } from './RouteMap';

const routePageMap: Record<string, string> = {
  sambodromo: '/carnival/sambodromo',
  centro: '/carnival/centro-lapa',
  orla: '/carnival/orla',
};

export function RouteCards() {
  const { language } = useLanguage();
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const t = (key: string) => translations[key]?.[language] || key;

  const toggleExpand = (id: string) => {
    setExpandedId(expandedId === id ? null : id);
  };

  return (
    <section data-routes-section>
      <motion.h2
        className="carnival-section-title"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5 }}
      >
        {t('routes.title')}
      </motion.h2>
      <motion.p
        className="carnival-section-subtitle"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5, delay: 0.1 }}
      >
        {t('routes.subtitle')}
      </motion.p>

      <div className="routes-layout">
        <div className="route-cards">
          {routes.map((route, index) => (
            <motion.div
              key={route.id}
              className={`route-card ${expandedId === route.id ? 'expanded' : ''}`}
              onClick={() => toggleExpand(route.id)}
              initial={{ opacity: 0, x: -30 }}
              whileInView={{ opacity: 1, x: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.5, delay: index * 0.1 }}
              whileHover={{ scale: 1.02 }}
              style={{ cursor: 'pointer' }}
            >
              <div
                className="route-chip"
                style={{ background: `${route.color}20`, borderLeft: `3px solid ${route.color}` }}
              >
                {t(route.subtitleKey)}
              </div>
              <div className="route-title">{t(route.titleKey)}</div>
              <p className="route-desc">{t(route.descriptionKey)}</p>
              <motion.div
                className="route-stops"
                initial={false}
                animate={{ height: expandedId === route.id ? 'auto' : 0, opacity: expandedId === route.id ? 1 : 0 }}
                transition={{ duration: 0.3 }}
              >
                <div className="route-line">
                  {route.stops.map((stop, i) => (
                    <span key={i}>
                      <span className="route-dot" style={{ background: route.color, boxShadow: `0 0 10px ${route.color}` }} />
                      {t(stop.name)}
                    </span>
                  ))}
                </div>
              </motion.div>
              <div className="route-expand-hint">
                {expandedId === route.id ? '▲' : '▼'}
              </div>
              <Link 
                to={routePageMap[route.id] || '/carnival/routes'}
                className="route-view-btn"
                onClick={(e) => e.stopPropagation()}
              >
                {t('routes.viewFull')}
              </Link>
            </motion.div>
          ))}
        </div>

        <motion.div
          className="route-map-container"
          initial={{ opacity: 0, scale: 0.9 }}
          whileInView={{ opacity: 1, scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6, delay: 0.2 }}
        >
          <RouteMap />
        </motion.div>
      </div>

      <style>{`
        .routes-layout {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
          gap: 2.5rem;
          align-items: center;
        }

        .route-cards {
          display: grid;
          gap: 1.4rem;
        }

        .route-card {
          padding: 1.8rem;
          background: rgba(6, 15, 37, 0.78);
          border-radius: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.1);
          transition: all 0.3s ease;
        }

        .route-card.expanded {
          border-color: rgba(255, 223, 0, 0.4);
        }

        .route-chip {
          display: inline-flex;
          align-items: center;
          gap: 0.4rem;
          font-size: 0.75rem;
          text-transform: uppercase;
          letter-spacing: 0.12em;
          font-weight: 700;
          padding: 0.35rem 0.7rem;
          border-radius: 999px;
          margin-bottom: 0.8rem;
        }

        .route-title {
          font-size: 1.3rem;
          color: #ffdf00;
          margin-bottom: 0.5rem;
          font-family: 'Fraunces', serif;
        }

        .route-desc {
          font-size: 0.95rem;
          color: rgba(255, 255, 255, 0.7);
          line-height: 1.5;
        }

        .route-stops {
          overflow: hidden;
          margin-top: 1rem;
        }

        .route-line {
          display: flex;
          align-items: center;
          gap: 0.6rem;
          flex-wrap: wrap;
          font-size: 0.9rem;
          color: rgba(255, 255, 255, 0.7);
        }

        .route-line span {
          display: inline-flex;
          align-items: center;
          gap: 0.4rem;
        }

        .route-dot {
          width: 8px;
          height: 8px;
          border-radius: 50%;
        }

        .route-expand-hint {
          text-align: center;
          margin-top: 0.8rem;
          font-size: 0.8rem;
          color: rgba(255, 255, 255, 0.5);
        }

        .route-view-btn {
          display: inline-block;
          margin-top: 1rem;
          padding: 0.6rem 1.2rem;
          background: rgba(255, 223, 0, 0.15);
          border: 1px solid rgba(255, 223, 0, 0.3);
          border-radius: 2rem;
          color: #ffdf00;
          text-decoration: none;
          font-size: 0.85rem;
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.05em;
          transition: all 0.3s ease;
        }

        .route-view-btn:hover {
          background: rgba(255, 223, 0, 0.25);
          transform: translateX(5px);
        }

        .route-map-container {
          position: relative;
        }
      `}</style>
    </section>
  );
}
