import { useState, useMemo } from 'react';
import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { scheduleEvents, scheduleTranslations } from '../data/schedule';
import type { ScheduleEvent } from '../types';

type Category = 'all' | 'desfile' | 'bloco' | 'baile' | 'cultural';

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.08 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.4 } },
};

export default function CarnivalSchedule() {
  const { language } = useLanguage();
  const [filter, setFilter] = useState<Category>('all');
  const t = (key: string) => {
    if (scheduleTranslations[key]) {
      return scheduleTranslations[key][language];
    }
    return translations[key]?.[language] || key;
  };

  const filteredEvents = useMemo(() => {
    if (filter === 'all') return scheduleEvents;
    return scheduleEvents.filter((e) => e.category === filter);
  }, [filter]);

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString(language === 'pt' ? 'pt-BR' : 'es-ES', {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
    });
  };

  const getCategoryColor = (category: ScheduleEvent['category']) => {
    const colors = {
      destaque: '#ffdf00',
      bloco: '#00b36b',
      baile: '#ff6b3d',
      cultural: '#0084c6',
    };
    return colors[category] || '#ffdf00';
  };

  const filters: { key: Category; label: string }[] = [
    { key: 'all', label: t('schedule.filter.all') },
    { key: 'desfile', label: t('schedule.filter.desfile') },
    { key: 'bloco', label: t('schedule.filter.bloco') },
    { key: 'baile', label: t('schedule.filter.baile') },
    { key: 'cultural', label: t('schedule.filter.cultural') },
  ];

  return (
    <motion.div
      data-carnival-schedule
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <div className="schedule-page">
        <motion.div className="schedule-page-header" variants={itemVariants}>
          <h1>{t('schedule.title')}</h1>
          <p>{t('schedule.subtitle')}</p>
        </motion.div>

        <motion.div className="schedule-filters" variants={itemVariants}>
          {filters.map((f) => (
            <motion.button
              key={f.key}
              className={`filter-btn ${filter === f.key ? 'active' : ''}`}
              onClick={() => setFilter(f.key)}
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
            >
              {f.label}
            </motion.button>
          ))}
        </motion.div>

        <motion.div className="schedule-events" variants={containerVariants}>
          {filteredEvents.length === 0 ? (
            <motion.p className="no-events" variants={itemVariants}>
              {t('schedule.noEvents')}
            </motion.p>
          ) : (
            filteredEvents.map((event) => (
              <motion.div
                key={event.id}
                className="event-card"
                variants={itemVariants}
                whileHover={{ scale: 1.01, y: -2 }}
                style={{ '--category-color': getCategoryColor(event.category) } as React.CSSProperties}
              >
                {event.isLive && (
                  <div className="event-live-badge">
                    <span className="live-dot" />
                    {t('schedule.liveNow')}
                  </div>
                )}
                <div className="event-date">
                  <span className="event-day">
                    {new Date(event.date).getDate()}
                  </span>
                  <span className="event-month">
                    {new Date(event.date).toLocaleDateString(
                      language === 'pt' ? 'pt-BR' : 'es-ES',
                      { month: 'short' }
                    )}
                  </span>
                </div>
                <div className="event-info">
                  <span
                    className="event-category"
                    style={{ color: getCategoryColor(event.category) }}
                  >
                    {event.category === 'desfile'
                      ? 'Desfile'
                      : event.category === 'bloco'
                        ? 'Bloco'
                        : event.category === 'baile'
                          ? 'Baile'
                          : 'Cultural'}
                  </span>
                  <h3>{t(event.titleKey)}</h3>
                  <p>{t(event.descriptionKey)}</p>
                  <div className="event-meta">
                    <span>📍 {event.location}</span>
                    <span>🕐 {event.time}</span>
                    {event.price && <span>🎟️ {event.price}</span>}
                  </div>
                </div>
                <div className="event-indicator" />
              </motion.div>
            ))
          )}
        </motion.div>
      </div>

      <style>{`
        .schedule-page {
          padding: 3rem 0;
        }

        .schedule-page-header {
          margin-bottom: 2rem;
        }

        .schedule-page-header h1 {
          font-family: 'Fraunces', serif;
          font-size: clamp(2rem, 4vw, 3rem);
          color: #ffdf00;
          margin-bottom: 0.5rem;
        }

        .schedule-page-header p {
          color: rgba(255, 255, 255, 0.7);
          max-width: 600px;
        }

        .schedule-filters {
          display: flex;
          gap: 0.8rem;
          margin-bottom: 2rem;
          flex-wrap: wrap;
        }

        .filter-btn {
          padding: 0.6rem 1.2rem;
          background: rgba(255, 255, 255, 0.08);
          border: 1px solid rgba(255, 255, 255, 0.1);
          border-radius: 999px;
          color: rgba(255, 255, 255, 0.7);
          font-weight: 600;
          cursor: pointer;
          transition: all 0.3s ease;
        }

        .filter-btn.active {
          background: linear-gradient(135deg, #ff00ff, #ff6b3d);
          color: white;
          border-color: transparent;
        }

        .schedule-events {
          display: flex;
          flex-direction: column;
          gap: 1rem;
        }

        .event-card {
          display: flex;
          gap: 1.5rem;
          padding: 1.5rem;
          background: rgba(6, 15, 37, 0.72);
          border-radius: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.08);
          transition: all 0.3s ease;
          cursor: pointer;
          position: relative;
        }

        .event-card:hover {
          border-color: var(--category-color);
        }

        .event-live-badge {
          position: absolute;
          top: -8px;
          right: 1rem;
          display: flex;
          align-items: center;
          gap: 0.4rem;
          background: linear-gradient(135deg, #ff00ff, #ff6b3d);
          padding: 0.3rem 0.8rem;
          border-radius: 999px;
          font-size: 0.7rem;
          font-weight: 700;
          text-transform: uppercase;
          letter-spacing: 0.1em;
          color: white;
        }

        .live-dot {
          width: 6px;
          height: 6px;
          border-radius: 50%;
          background: white;
          animation: livePulse 1s ease-in-out infinite;
        }

        @keyframes livePulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.4; }
        }

        .event-date {
          display: flex;
          flex-direction: column;
          align-items: center;
          justify-content: center;
          min-width: 70px;
          padding: 0.8rem;
          background: rgba(255, 255, 255, 0.05);
          border-radius: 1.5rem;
        }

        .event-day {
          font-size: 1.8rem;
          font-weight: 700;
          color: #ffdf00;
          line-height: 1;
        }

        .event-month {
          font-size: 0.75rem;
          text-transform: uppercase;
          color: rgba(255, 255, 255, 0.6);
        }

        .event-info {
          flex: 1;
        }

        .event-category {
          font-size: 0.75rem;
          text-transform: uppercase;
          letter-spacing: 0.1em;
          font-weight: 700;
        }

        .event-info h3 {
          font-size: 1.2rem;
          color: #f6f1e5;
          margin: 0.3rem 0;
        }

        .event-info p {
          font-size: 0.9rem;
          color: rgba(255, 255, 255, 0.6);
          margin-bottom: 0.6rem;
        }

        .event-meta {
          display: flex;
          gap: 1rem;
          flex-wrap: wrap;
          font-size: 0.85rem;
          color: rgba(255, 255, 255, 0.7);
        }

        .event-indicator {
          width: 4px;
          border-radius: 2px;
          background: var(--category-color);
        }

        .no-events {
          text-align: center;
          padding: 3rem;
          color: rgba(255, 255, 255, 0.5);
        }

        @media (max-width: 600px) {
          .event-card {
            flex-direction: column;
          }

          .event-date {
            flex-direction: row;
            gap: 0.5rem;
            min-width: auto;
          }

          .event-indicator {
            display: none;
          }
        }
      `}</style>
    </motion.div>
  );
}
