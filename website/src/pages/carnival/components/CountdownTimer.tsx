import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import type { CountdownData } from '../types';

const CARNIVAL_DATE = new Date('2026-02-14T20:00:00');

export function CountdownTimer() {
  const { language } = useLanguage();
  const [countdown, setCountdown] = useState<CountdownData>({ days: 0, hours: 0, minutes: 0, seconds: 0 });
  const t = (key: string) => translations[key]?.[language] || key;

  useEffect(() => {
    const updateCountdown = () => {
      const now = new Date();
      const diff = CARNIVAL_DATE.getTime() - now.getTime();

      if (diff <= 0) {
        setCountdown({ days: 0, hours: 0, minutes: 0, seconds: 0 });
        return;
      }

      const days = Math.floor(diff / (1000 * 60 * 60 * 24));
      const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
      const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
      const seconds = Math.floor((diff % (1000 * 60)) / 1000);

      setCountdown({ days, hours, minutes, seconds });
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);
    return () => clearInterval(interval);
  }, []);

  const timeUnits = [
    { value: countdown.days, label: t('countdown.days') },
    { value: countdown.hours, label: t('countdown.hours') },
    { value: countdown.minutes, label: t('countdown.minutes') },
    { value: countdown.seconds, label: t('countdown.seconds') },
  ];

  return (
    <div data-countdown-timer className="countdown-timer">
      <p className="countdown-label">{t('countdown.title')}</p>
      <div className="countdown-units">
        {timeUnits.map((unit, index) => (
          <motion.div
            key={unit.label}
            className="countdown-unit"
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: index * 0.1 }}
          >
            <span className="countdown-value">{String(unit.value).padStart(2, '0')}</span>
            <span className="countdown-unit-label">{unit.label}</span>
          </motion.div>
        ))}
      </div>
      <style>{`
        .countdown-timer {
          padding: 1.5rem;
          background: rgba(6, 15, 37, 0.85);
          border-radius: 2rem;
          border: 1px solid rgba(255, 223, 0, 0.3);
          text-align: center;
        }

        .countdown-label {
          font-size: 0.85rem;
          text-transform: uppercase;
          letter-spacing: 0.15em;
          color: rgba(255, 255, 255, 0.6);
          margin-bottom: 1rem;
        }

        .countdown-units {
          display: flex;
          justify-content: center;
          gap: 1rem;
        }

        .countdown-unit {
          display: flex;
          flex-direction: column;
          align-items: center;
          min-width: 60px;
        }

        .countdown-value {
          font-family: 'Fraunces', serif;
          font-size: 2.5rem;
          font-weight: 700;
          color: #ffdf00;
          line-height: 1;
          text-shadow: 0 0 20px rgba(255, 223, 0, 0.5);
        }

        .countdown-unit-label {
          font-size: 0.7rem;
          text-transform: uppercase;
          letter-spacing: 0.1em;
          color: rgba(255, 255, 255, 0.5);
          margin-top: 0.3rem;
        }

        @media (max-width: 480px) {
          .countdown-units {
            gap: 0.5rem;
          }

          .countdown-value {
            font-size: 1.8rem;
          }

          .countdown-unit {
            min-width: 45px;
          }
        }
      `}</style>
    </div>
  );
}
