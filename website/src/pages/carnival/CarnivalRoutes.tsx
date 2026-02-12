import { useState } from 'react';
import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { routes } from '../data/routes';

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

export default function CarnivalRoutes() {
  const { language } = useLanguage();
  const [selectedRoute, setSelectedRoute] = useState(routes[0].id);
  const t = (key: string) => translations[key]?.[language] || key;

  const route = routes.find((r) => r.id === selectedRoute)!;

  return (
    <motion.div
      data-carnival-routes
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <div className="routes-page">
        <motion.div className="routes-page-header" variants={itemVariants}>
          <h1>{t('routes.title')}</h1>
          <p>{t('routes.subtitle')}</p>
        </motion.div>

        <div className="routes-page-content">
          <motion.div className="routes-page-sidebar" variants={itemVariants}>
            <h3>Rotas</h3>
            <div className="routes-list">
              {routes.map((r) => (
                <motion.button
                  key={r.id}
                  className={`route-list-item ${selectedRoute === r.id ? 'active' : ''}`}
                  onClick={() => setSelectedRoute(r.id)}
                  whileHover={{ scale: 1.02 }}
                  whileTap={{ scale: 0.98 }}
                  style={{ '--route-color': r.color } as React.CSSProperties}
                >
                  <div className="route-list-indicator" />
                  <div className="route-list-content">
                    <span className="route-list-type">{t(r.subtitleKey)}</span>
                    <span className="route-list-title">{t(r.titleKey)}</span>
                  </div>
                </motion.button>
              ))}
            </div>
          </motion.div>

          <motion.div
            className="routes-page-detail"
            key={selectedRoute}
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.4 }}
          >
            <div
              className="route-detail-header"
              style={{ borderLeftColor: route.color }}
            >
              <span className="route-detail-type">{t(route.subtitleKey)}</span>
              <h2>{t(route.titleKey)}</h2>
              <p>{t(route.descriptionKey)}</p>
            </div>

            <div className="route-detail-stops">
              <h3>Paradas</h3>
              <div className="stops-list">
                {route.stops.map((stop, index) => (
                  <motion.div
                    key={stop.name}
                    className="stop-item"
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: index * 0.1 }}
                  >
                    <div
                      className="stop-marker"
                      style={{
                        background: route.color,
                        boxShadow: `0 0 12px ${route.color}`,
                      }}
                    >
                      {stop.order}
                    </div>
                    <span>{t(stop.name)}</span>
                  </motion.div>
                ))}
              </div>
            </div>

            <div className="route-detail-map">
              <svg viewBox="0 0 420 280" fill="none" xmlns="http://www.w3.org/2000/svg">
                <defs>
                  <linearGradient
                    id={`detail-${route.id}`}
                    x1="40"
                    y1="220"
                    x2="360"
                    y2="150"
                    gradientUnits="userSpaceOnUse"
                  >
                    <stop stopColor={route.color} />
                    <stop offset="1" stopColor={route.color} stopOpacity="0.4" />
                  </linearGradient>
                </defs>
                <motion.path
                  d={route.pathData}
                  stroke={`url(#detail-${route.id})`}
                  strokeWidth="6"
                  strokeLinecap="round"
                  fill="none"
                  initial={{ pathLength: 0 }}
                  animate={{ pathLength: 1 }}
                  transition={{ duration: 1.5, ease: 'easeInOut' }}
                />
                {route.stops.map((stop, index) => (
                  <motion.circle
                    key={stop.name}
                    cx={40 + index * 160}
                    cy={220 - index * 75}
                    r="8"
                    fill={route.color}
                    initial={{ scale: 0 }}
                    animate={{ scale: 1 }}
                    transition={{ delay: index * 0.3 + 0.5 }}
                  />
                ))}
              </svg>
            </div>

            <div className="route-detail-tips">
              <h3>Dicas</h3>
              <ul>
                <li>Chegue 30 minutos antes do início</li>
                <li>Use roupas confortáveis</li>
                <li>Mantenha-se hidratado</li>
                <li>Guarde objetos de valor em pochete</li>
              </ul>
            </div>
          </motion.div>
        </div>
      </div>

      <style>{`
        .routes-page {
          padding: 3rem 0;
        }

        .routes-page-header {
          margin-bottom: 3rem;
        }

        .routes-page-header h1 {
          font-family: 'Fraunces', serif;
          font-size: clamp(2rem, 4vw, 3rem);
          color: #ffdf00;
          margin-bottom: 0.5rem;
        }

        .routes-page-header p {
          color: rgba(255, 255, 255, 0.7);
          max-width: 600px;
        }

        .routes-page-content {
          display: grid;
          grid-template-columns: 280px 1fr;
          gap: 2rem;
        }

        .routes-page-sidebar {
          position: sticky;
          top: 100px;
          height: fit-content;
        }

        .routes-page-sidebar h3 {
          font-size: 0.85rem;
          text-transform: uppercase;
          letter-spacing: 0.15em;
          color: rgba(255, 255, 255, 0.5);
          margin-bottom: 1rem;
        }

        .routes-list {
          display: flex;
          flex-direction: column;
          gap: 0.8rem;
        }

        .route-list-item {
          display: flex;
          align-items: center;
          gap: 1rem;
          padding: 1rem;
          background: rgba(6, 15, 37, 0.72);
          border: 1px solid rgba(255, 255, 255, 0.08);
          border-radius: 2rem;
          cursor: pointer;
          transition: all 0.3s ease;
          text-align: left;
        }

        .route-list-item.active {
          border-color: var(--route-color);
          background: rgba(6, 15, 37, 0.9);
        }

        .route-list-indicator {
          width: 4px;
          height: 40px;
          border-radius: 2px;
          background: var(--route-color);
        }

        .route-list-content {
          display: flex;
          flex-direction: column;
        }

        .route-list-type {
          font-size: 0.75rem;
          text-transform: uppercase;
          letter-spacing: 0.1em;
          color: var(--route-color);
        }

        .route-list-title {
          font-size: 1rem;
          color: #f6f1e5;
          font-weight: 600;
        }

        .routes-page-detail {
          background: rgba(6, 15, 37, 0.65);
          border-radius: 2rem;
          padding: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .route-detail-header {
          padding-left: 1.5rem;
          border-left: 4px solid;
          margin-bottom: 2rem;
        }

        .route-detail-type {
          font-size: 0.8rem;
          text-transform: uppercase;
          letter-spacing: 0.15em;
          color: rgba(255, 255, 255, 0.6);
        }

        .route-detail-header h2 {
          font-family: 'Fraunces', serif;
          font-size: 1.8rem;
          color: #ffdf00;
          margin: 0.5rem 0;
        }

        .route-detail-header p {
          color: rgba(255, 255, 255, 0.75);
          line-height: 1.6;
        }

        .route-detail-stops {
          margin-bottom: 2rem;
        }

        .route-detail-stops h3 {
          font-size: 1rem;
          color: rgba(255, 255, 255, 0.8);
          margin-bottom: 1rem;
        }

        .stops-list {
          display: flex;
          gap: 1rem;
          flex-wrap: wrap;
        }

        .stop-item {
          display: flex;
          align-items: center;
          gap: 0.6rem;
          background: rgba(255, 255, 255, 0.05);
          padding: 0.6rem 1rem;
          border-radius: 2rem;
        }

        .stop-marker {
          width: 24px;
          height: 24px;
          border-radius: 50%;
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 0.75rem;
          font-weight: 700;
          color: #060f25;
        }

        .route-detail-map {
          margin-bottom: 2rem;
          border-radius: 1.5rem;
          overflow: hidden;
          background: rgba(0, 0, 0, 0.2);
        }

        .route-detail-map svg {
          width: 100%;
          height: auto;
        }

        .route-detail-tips h3 {
          font-size: 1rem;
          color: rgba(255, 255, 255, 0.8);
          margin-bottom: 1rem;
        }

        .route-detail-tips ul {
          list-style: none;
          display: grid;
          gap: 0.6rem;
        }

        .route-detail-tips li {
          color: rgba(255, 255, 255, 0.7);
          display: flex;
          align-items: center;
          gap: 0.6rem;
        }

        .route-detail-tips li::before {
          content: '✓';
          color: #ffdf00;
        }

        @media (max-width: 800px) {
          .routes-page-content {
            grid-template-columns: 1fr;
          }

          .routes-page-sidebar {
            position: static;
          }

          .routes-list {
            flex-direction: row;
            overflow-x: auto;
            padding-bottom: 0.5rem;
          }

          .route-list-item {
            min-width: 200px;
          }
        }
      `}</style>
    </motion.div>
  );
}
