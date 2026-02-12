import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';

const highlights = [
  {
    icon: '🥁',
    titleKey: 'highlights.samba.title',
    descKey: 'highlights.samba.desc',
  },
  {
    icon: '🎺',
    titleKey: 'highlights.bloco.title',
    descKey: 'highlights.bloco.desc',
  },
  {
    icon: '💃',
    titleKey: 'highlights.ball.title',
    descKey: 'highlights.ball.desc',
  },
  {
    icon: '🍹',
    titleKey: 'highlights.food.title',
    descKey: 'highlights.food.desc',
  },
];

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 30 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.5, ease: 'easeOut' },
  },
};

export function Highlights() {
  const { language } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  return (
    <section data-highlights-section>
      <motion.h2
        className="carnival-section-title"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5 }}
      >
        {t('highlights.title')}
      </motion.h2>
      <motion.p
        className="carnival-section-subtitle"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5, delay: 0.1 }}
      >
        {t('highlights.subtitle')}
      </motion.p>
      <motion.div
        className="carnival-cards"
        variants={containerVariants}
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, margin: '-50px' }}
      >
        {highlights.map((item, index) => (
          <motion.div
            key={index}
            className="carnival-card"
            variants={itemVariants}
            whileHover={{ y: -6, transition: { duration: 0.2 } }}
          >
            <div className="carnival-card-icon">{item.icon}</div>
            <h3>{t(item.titleKey)}</h3>
            <p>{t(item.descKey)}</p>
          </motion.div>
        ))}
      </motion.div>
      <style>{`
        .carnival-section-title {
          font-family: 'Fraunces', serif;
          font-size: clamp(2rem, 3vw, 3rem);
          color: #ffdf00;
          margin-bottom: 0.8rem;
        }

        .carnival-section-subtitle {
          color: rgba(255, 255, 255, 0.7);
          max-width: 700px;
          margin-bottom: 2.5rem;
        }

        .carnival-cards {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
          gap: 1.8rem;
        }

        .carnival-card {
          padding: 1.8rem;
          background: rgba(6, 15, 37, 0.72);
          border-radius: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.08);
          box-shadow: 0 18px 30px rgba(0, 0, 0, 0.2);
          transition: border-color 0.3s ease;
        }

        .carnival-card:hover {
          border-color: rgba(255, 223, 0, 0.4);
        }

        .carnival-card-icon {
          font-size: 2.2rem;
          margin-bottom: 0.7rem;
        }

        .carnival-card h3 {
          color: #ffdf00;
          margin-bottom: 0.6rem;
          font-size: 1.15rem;
        }

        .carnival-card p {
          color: rgba(255, 255, 255, 0.75);
          line-height: 1.5;
        }
      `}</style>
    </section>
  );
}
