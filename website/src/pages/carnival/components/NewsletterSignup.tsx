import { useState } from 'react';
import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';

export function NewsletterSignup() {
  const { language } = useLanguage();
  const [email, setEmail] = useState('');
  const [subscribed, setSubscribed] = useState(false);
  const t = (key: string) => translations[key]?.[language] || key;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (email) {
      setSubscribed(true);
      setEmail('');
    }
  };

  return (
    <div data-newsletter-signup className="newsletter-signup">
      {!subscribed ? (
        <>
          <h3>{t('newsletter.title')}</h3>
          <p>{t('newsletter.subtitle')}</p>
          <form onSubmit={handleSubmit} className="newsletter-form">
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t('newsletter.email')}
              className="newsletter-input"
              required
            />
            <motion.button
              type="submit"
              className="newsletter-button"
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
            >
              {t('newsletter.subscribe')}
            </motion.button>
          </form>
        </>
      ) : (
        <motion.div
          className="newsletter-success"
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
        >
          <span className="success-icon">🎉</span>
          <p>{t('newsletter.success')}</p>
        </motion.div>
      )}
      <style>{`
        .newsletter-signup {
          padding: 2rem;
          background: rgba(6, 15, 37, 0.85);
          border-radius: 2rem;
          border: 1px solid rgba(255, 223, 0, 0.3);
          text-align: center;
        }

        .newsletter-signup h3 {
          font-family: 'Fraunces', serif;
          font-size: 1.5rem;
          color: #ffdf00;
          margin-bottom: 0.5rem;
        }

        .newsletter-signup p {
          color: rgba(255, 255, 255, 0.7);
          font-size: 0.9rem;
          margin-bottom: 1.5rem;
        }

        .newsletter-form {
          display: flex;
          gap: 0.8rem;
        }

        .newsletter-input {
          flex: 1;
          padding: 0.8rem 1.2rem;
          background: rgba(255, 255, 255, 0.1);
          border: 1px solid rgba(255, 255, 255, 0.2);
          border-radius: 2rem;
          color: white;
          font-size: 0.95rem;
          outline: none;
          transition: border-color 0.3s ease;
        }

        .newsletter-input::placeholder {
          color: rgba(255, 255, 255, 0.5);
        }

        .newsletter-input:focus {
          border-color: #ffdf00;
        }

        .newsletter-button {
          padding: 0.8rem 1.5rem;
          background: linear-gradient(135deg, #ffdf00, #ff6b3d);
          border: none;
          border-radius: 2rem;
          color: #060f25;
          font-weight: 700;
          font-size: 0.9rem;
          cursor: pointer;
          transition: box-shadow 0.3s ease;
        }

        .newsletter-button:hover {
          box-shadow: 0 5px 20px rgba(255, 223, 0, 0.3);
        }

        .newsletter-success {
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 0.5rem;
        }

        .success-icon {
          font-size: 2.5rem;
        }

        .newsletter-success p {
          color: #ffdf00;
          margin: 0;
        }

        @media (max-width: 480px) {
          .newsletter-form {
            flex-direction: column;
          }

          .newsletter-button {
            width: 100%;
          }
        }
      `}</style>
    </div>
  );
}
