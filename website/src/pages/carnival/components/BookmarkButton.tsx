import { useState } from 'react';
import { motion } from 'framer-motion';
import { translations } from '../translations';
import { useLanguage } from '../hooks/useLanguage';

interface BookmarkButtonProps {
  routeId?: string;
}

export function BookmarkButton({ routeId }: BookmarkButtonProps) {
  const { language } = useLanguage();
  const [bookmarked, setBookmarked] = useState(false);
  const t = (key: string) => translations[key]?.[language] || key;

  const toggleBookmark = () => {
    setBookmarked(!bookmarked);
  };

  return (
    <motion.button
      data-bookmark-button
      className={`bookmark-button ${bookmarked ? 'bookmarked' : ''}`}
      onClick={toggleBookmark}
      whileHover={{ scale: 1.05 }}
      whileTap={{ scale: 0.95 }}
      aria-pressed={bookmarked}
    >
      <span className="bookmark-icon">{bookmarked ? '★' : '☆'}</span>
      <span className="bookmark-label">
        {bookmarked ? t('routes.saved') : t('routes.save')}
      </span>
      <style>{`
        .bookmark-button {
          display: inline-flex;
          align-items: center;
          gap: 0.5rem;
          padding: 0.7rem 1.2rem;
          background: rgba(255, 255, 255, 0.1);
          border: 1px solid rgba(255, 255, 255, 0.2);
          border-radius: 2rem;
          color: rgba(255, 255, 255, 0.8);
          font-size: 0.9rem;
          font-weight: 600;
          cursor: pointer;
          transition: all 0.3s ease;
        }

        .bookmark-button:hover {
          background: rgba(255, 223, 0, 0.15);
          border-color: rgba(255, 223, 0, 0.4);
        }

        .bookmark-button.bookmarked {
          background: rgba(255, 223, 0, 0.2);
          border-color: rgba(255, 223, 0, 0.5);
          color: #ffdf00;
        }

        .bookmark-icon {
          font-size: 1.1rem;
        }

        .bookmark-label {
          text-transform: uppercase;
          letter-spacing: 0.05em;
        }
      `}</style>
    </motion.button>
  );
}
