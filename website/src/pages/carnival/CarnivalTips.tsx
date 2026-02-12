import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';

const tips = [
  {
    icon: '🚇',
    titleKey: 'tips.transport.title',
    descKey: 'tips.transport.desc',
  },
  {
    icon: '🔒',
    titleKey: 'tips.safety.title',
    descKey: 'tips.safety.desc',
  },
  {
    icon: '💊',
    titleKey: 'tips.health.title',
    descKey: 'tips.health.desc',
  },
  {
    icon: '👘',
    titleKey: 'tips.costume.title',
    descKey: 'tips.costume.desc',
  },
];

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.1 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 30 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.5 } },
};

export default function CarnivalTips() {
  const { language } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  return (
    <motion.div
      data-carnival-tips
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <div className="tips-page">
        <motion.div className="tips-page-header" variants={itemVariants}>
          <h1>{t('tips.title')}</h1>
          <p>{t('tips.subtitle')}</p>
        </motion.div>

        <div className="tips-grid">
          {tips.map((tip, index) => (
            <motion.div
              key={tip.titleKey}
              className="tip-card"
              variants={itemVariants}
              whileHover={{ scale: 1.02, y: -4 }}
              transition={{ type: 'spring', stiffness: 300 }}
            >
              <div className="tip-icon">{tip.icon}</div>
              <div className="tip-content">
                <h3>{t(tip.titleKey)}</h3>
                <p>{t(tip.descKey)}</p>
              </div>
            </motion.div>
          ))}
        </div>

        <motion.div className="tips-extra" variants={itemVariants}>
          <h2>Informações adicionais</h2>
          <div className="tips-extra-grid">
            <div className="extra-card">
              <h4>🏥 Postos de saúde</h4>
              <ul>
                <li>Hospital Souza Aguiar - Centro</li>
                <li>Hospital Copa D'Or - Copacabana</li>
                <li>Samu: 192</li>
              </ul>
            </div>
            <div className="extra-card">
              <h4>🚨 Emergências</h4>
              <ul>
                <li>Polícia: 190</li>
                <li>Defesa Civil: 199</li>
                <li>Guarda Municipal: 1746</li>
              </ul>
            </div>
            <div className="extra-card">
              <h4>📱 Apps úteis</h4>
              <ul>
                <li>Metrô Rio (horários)</li>
                <li>99 App (transporte)</li>
                <li>Google Maps (rotas)</li>
              </ul>
            </div>
            <div className="extra-card">
              <h4>💰 Custos médios</h4>
              <ul>
                <li>Metrô: R$ 7,50</li>
                <li>Bloco: Grátis</li>
                <li>Desfile: R$ 80-800</li>
              </ul>
            </div>
          </div>
        </motion.div>

        <motion.div className="tips-emergency" variants={itemVariants}>
          <div className="emergency-icon">⚠️</div>
          <div className="emergency-content">
            <h3>Números de emergência</h3>
            <div className="emergency-numbers">
              <a href="tel:190" className="emergency-btn">
                <span>190</span>
                <span>Polícia</span>
              </a>
              <a href="tel:192" className="emergency-btn">
                <span>192</span>
                <span>Samu</span>
              </a>
              <a href="tel:193" className="emergency-btn">
                <span>193</span>
                <span>Bombeiros</span>
              </a>
            </div>
          </div>
        </motion.div>
      </div>

      <style>{`
        .tips-page {
          padding: 3rem 0;
        }

        .tips-page-header {
          margin-bottom: 3rem;
        }

        .tips-page-header h1 {
          font-family: 'Fraunces', serif;
          font-size: clamp(2rem, 4vw, 3rem);
          color: #ffdf00;
          margin-bottom: 0.5rem;
        }

        .tips-page-header p {
          color: rgba(255, 255, 255, 0.7);
          max-width: 600px;
        }

        .tips-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
          gap: 1.5rem;
          margin-bottom: 3rem;
        }

        .tip-card {
          display: flex;
          gap: 1.2rem;
          padding: 1.8rem;
          background: rgba(6, 15, 37, 0.72);
          border-radius: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.08);
          transition: border-color 0.3s ease;
        }

        .tip-card:hover {
          border-color: rgba(255, 223, 0, 0.4);
        }

        .tip-icon {
          font-size: 2.5rem;
          flex-shrink: 0;
        }

        .tip-content h3 {
          font-size: 1.15rem;
          color: #ffdf00;
          margin-bottom: 0.5rem;
        }

        .tip-content p {
          font-size: 0.95rem;
          color: rgba(255, 255, 255, 0.75);
          line-height: 1.5;
        }

        .tips-extra {
          margin-bottom: 3rem;
        }

        .tips-extra h2 {
          font-family: 'Fraunces', serif;
          font-size: 1.8rem;
          color: #ffdf00;
          margin-bottom: 1.5rem;
        }

        .tips-extra-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
          gap: 1.2rem;
        }

        .extra-card {
          padding: 1.5rem;
          background: rgba(6, 15, 37, 0.6);
          border-radius: 2rem;
          border: 1px solid rgba(255, 255, 255, 0.08);
        }

        .extra-card h4 {
          font-size: 1rem;
          color: #ffdf00;
          margin-bottom: 1rem;
        }

        .extra-card ul {
          list-style: none;
          display: flex;
          flex-direction: column;
          gap: 0.5rem;
        }

        .extra-card li {
          font-size: 0.9rem;
          color: rgba(255, 255, 255, 0.7);
        }

        .tips-emergency {
          display: flex;
          align-items: center;
          gap: 1.5rem;
          padding: 2rem;
          background: rgba(255, 107, 61, 0.15);
          border-radius: 2rem;
          border: 1px solid rgba(255, 107, 61, 0.3);
        }

        .emergency-icon {
          font-size: 3rem;
        }

        .emergency-content h3 {
          font-size: 1.2rem;
          color: #ffdf00;
          margin-bottom: 1rem;
        }

        .emergency-numbers {
          display: flex;
          gap: 1rem;
          flex-wrap: wrap;
        }

        .emergency-btn {
          display: flex;
          flex-direction: column;
          align-items: center;
          padding: 1rem 1.5rem;
          background: rgba(255, 107, 61, 0.2);
          border-radius: 1.5rem;
          text-decoration: none;
          transition: all 0.3s ease;
        }

        .emergency-btn:hover {
          background: rgba(255, 107, 61, 0.4);
          transform: scale(1.05);
        }

        .emergency-btn span:first-child {
          font-size: 1.5rem;
          font-weight: 700;
          color: #fff;
        }

        .emergency-btn span:last-child {
          font-size: 0.8rem;
          color: rgba(255, 255, 255, 0.7);
        }

        @media (max-width: 600px) {
          .tips-emergency {
            flex-direction: column;
            text-align: center;
          }

          .emergency-numbers {
            justify-content: center;
          }
        }
      `}</style>
    </motion.div>
  );
}
