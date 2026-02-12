import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { planImage } from '../data/gallery';

const planCards = [
  {
    titleKey: 'plan.transport.title',
    itemsKey: 'plan.transport.items',
  },
  {
    titleKey: 'plan.pack.title',
    itemsKey: 'plan.pack.items',
  },
  {
    titleKey: 'plan.stay.title',
    itemsKey: 'plan.stay.items',
  },
];

export function PlanSection() {
  const { language } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  const getItems = (itemsKey: string) => {
    const itemsStr = t(itemsKey);
    return itemsStr.split('|');
  };

  return (
    <section data-plan-section>
      <motion.h2
        className="carnival-section-title"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5 }}
      >
        {t('plan.title')}
      </motion.h2>
      <motion.p
        className="carnival-section-subtitle"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5, delay: 0.1 }}
      >
        {t('plan.subtitle')}
      </motion.p>

      <div className="plan-layout">
        <div className="plan-cards">
          {planCards.map((card, index) => (
            <motion.div
              key={card.titleKey}
              className="plan-card"
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.5, delay: index * 0.1 }}
            >
              <h3>{t(card.titleKey)}</h3>
              <ul>
                {getItems(card.itemsKey).map((item, i) => (
                  <li key={i}>{item}</li>
                ))}
              </ul>
            </motion.div>
          ))}
        </div>

        <motion.div
          className="plan-photo"
          initial={{ opacity: 0, x: 30 }}
          whileInView={{ opacity: 1, x: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6 }}
        >
          <img src={planImage} alt="Orla do Rio" />
        </motion.div>
      </div>

      <style>{`
        .plan-layout {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
          gap: 2.5rem;
          align-items: center;
        }

        .plan-cards {
          display: grid;
          gap: 1.4rem;
        }

        .plan-card {
          background: rgba(6, 15, 37, 0.72);
          border-radius: 2rem;
          padding: 1.6rem;
          border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .plan-card h3 {
          color: #ffdf00;
          margin-bottom: 0.7rem;
        }

        .plan-card ul {
          list-style: none;
          display: grid;
          gap: 0.6rem;
          font-size: 0.95rem;
          color: rgba(255, 255, 255, 0.75);
        }

        .plan-card li::before {
          content: '✦';
          color: #ffdf00;
          margin-right: 0.6rem;
        }

        .plan-photo {
          border-radius: 2rem;
          overflow: hidden;
          box-shadow: 0 30px 60px rgba(0, 0, 0, 0.35);
        }

        .plan-photo img {
          width: 100%;
          height: auto;
          display: block;
        }
      `}</style>
    </section>
  );
}
