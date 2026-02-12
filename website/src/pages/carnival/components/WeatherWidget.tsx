import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';

export function WeatherWidget() {
  const { language } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  const weatherData = {
    temperature: 32,
    condition: t('weather.sunny'),
    humidity: 75,
    icon: '☀️',
  };

  return (
    <div data-weather-widget className="weather-widget">
      <div className="weather-header">
        <span className="weather-icon">{weatherData.icon}</span>
        <span className="weather-title">{t('weather.title')}</span>
      </div>
      <div className="weather-info">
        <div className="weather-main">
          <span className="weather-temp">{weatherData.temperature}°</span>
          <span className="weather-condition">{weatherData.condition}</span>
        </div>
        <div className="weather-details">
          <span className="weather-humidity">💧 {weatherData.humidity}%</span>
        </div>
      </div>
      <style>{`
        .weather-widget {
          padding: 1rem 1.3rem;
          background: rgba(6, 15, 37, 0.85);
          border-radius: 2rem;
          border: 1px solid rgba(255, 223, 0, 0.3);
        }

        .weather-header {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          margin-bottom: 0.8rem;
        }

        .weather-icon {
          font-size: 1.5rem;
        }

        .weather-title {
          font-size: 0.75rem;
          text-transform: uppercase;
          letter-spacing: 0.1em;
          color: rgba(255, 255, 255, 0.6);
        }

        .weather-info {
          display: flex;
          align-items: center;
          justify-content: space-between;
        }

        .weather-main {
          display: flex;
          flex-direction: column;
        }

        .weather-temp {
          font-family: 'Fraunces', serif;
          font-size: 2rem;
          font-weight: 700;
          color: #ffdf00;
          line-height: 1;
        }

        .weather-condition {
          font-size: 0.8rem;
          color: rgba(255, 255, 255, 0.7);
        }

        .weather-humidity {
          font-size: 0.85rem;
          color: rgba(255, 255, 255, 0.6);
        }
      `}</style>
    </div>
  );
}
