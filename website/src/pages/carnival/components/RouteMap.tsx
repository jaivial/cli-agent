import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { routes } from '../data/routes';

export function RouteMap() {
  const { language } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  const getLegendLabel = (key: string) => {
    const labels: Record<string, string> = {
      Sambódromo: t('routes.legend.sambodromo'),
      'Centro + Lapa': t('routes.legend.centro'),
      Orla: t('routes.legend.orla'),
    };
    return labels[key] || key;
  };

  return (
    <div data-route-map className="route-map">
      <svg viewBox="0 0 420 280" fill="none" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <linearGradient id="route1" x1="40" y1="220" x2="360" y2="150" gradientUnits="userSpaceOnUse">
            <stop stopColor="#ffdf00" />
            <stop offset="1" stopColor="#ff6b3d" />
          </linearGradient>
          <linearGradient id="route2" x1="60" y1="70" x2="360" y2="80" gradientUnits="userSpaceOnUse">
            <stop stopColor="#00b36b" />
            <stop offset="1" stopColor="#0084c6" />
          </linearGradient>
          <linearGradient id="route3" x1="50" y1="150" x2="360" y2="210" gradientUnits="userSpaceOnUse">
            <stop stopColor="#0084c6" />
            <stop offset="1" stopColor="#ffdf00" />
          </linearGradient>
        </defs>

        {routes.map((route, index) => (
          <motion.path
            key={route.id}
            d={route.pathData}
            stroke={`url(#${route.gradientId})`}
            strokeWidth="6"
            strokeLinecap="round"
            fill="none"
            initial={{ pathLength: 0, opacity: 0 }}
            whileInView={{ pathLength: 1, opacity: 1 }}
            viewport={{ once: true }}
            transition={{ duration: 1.5, delay: index * 0.3, ease: 'easeInOut' }}
          />
        ))}

        <motion.circle
          cx="40"
          cy="220"
          r="8"
          fill="#ffdf00"
          initial={{ scale: 0 }}
          whileInView={{ scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4, delay: 0.5 }}
        />
        <motion.circle
          cx="360"
          cy="150"
          r="8"
          fill="#ff6b3d"
          initial={{ scale: 0 }}
          whileInView={{ scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4, delay: 0.7 }}
        />
        <motion.circle
          cx="60"
          cy="70"
          r="8"
          fill="#00b36b"
          initial={{ scale: 0 }}
          whileInView={{ scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4, delay: 0.9 }}
        />
        <motion.circle
          cx="360"
          cy="80"
          r="8"
          fill="#00b36b"
          initial={{ scale: 0 }}
          whileInView={{ scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4, delay: 1.1 }}
        />
        <motion.circle
          cx="50"
          cy="150"
          r="8"
          fill="#0084c6"
          initial={{ scale: 0 }}
          whileInView={{ scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4, delay: 1.3 }}
        />
        <motion.circle
          cx="360"
          cy="210"
          r="8"
          fill="#ffdf00"
          initial={{ scale: 0 }}
          whileInView={{ scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4, delay: 1.5 }}
        />
      </svg>

      <div className="route-legend">
        <span>
          <span className="legend-dot" style={{ background: '#ffdf00' }} />
          {getLegendLabel('Sambódromo')}
        </span>
        <span>
          <span className="legend-dot" style={{ background: '#00b36b' }} />
          {getLegendLabel('Centro + Lapa')}
        </span>
        <span>
          <span className="legend-dot" style={{ background: '#0084c6' }} />
          {getLegendLabel('Orla')}
        </span>
      </div>
      <p className="route-note">{t('routes.note')}</p>

      <style>{`
        .route-map {
          position: relative;
          padding: 2rem;
          border-radius: 2rem;
          background: rgba(6, 15, 37, 0.65);
          border: 1px solid rgba(255, 255, 255, 0.1);
          overflow: hidden;
        }

        .route-map svg {
          width: 100%;
          height: auto;
        }

        .route-legend {
          display: grid;
          gap: 0.7rem;
          margin-top: 1.5rem;
          font-size: 0.9rem;
        }

        .route-legend span {
          display: inline-flex;
          align-items: center;
          gap: 0.6rem;
        }

        .legend-dot {
          width: 10px;
          height: 10px;
          border-radius: 50%;
        }

        .route-note {
          font-size: 0.85rem;
          color: rgba(255, 255, 255, 0.6);
          margin-top: 1rem;
        }
      `}</style>
    </div>
  );
}
