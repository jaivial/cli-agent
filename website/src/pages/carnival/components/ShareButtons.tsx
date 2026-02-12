import { useState } from 'react';
import { motion } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';

interface ShareButtonsProps {
  url?: string;
  title?: string;
}

export function ShareButtons({ url = window.location.href, title = 'Carnaval Rio 2026' }: ShareButtonsProps) {
  const { language } = useLanguage();
  const [copied, setCopied] = useState(false);
  const t = (key: string) => translations[key]?.[language] || key;

  const shareLinks = [
    {
      name: 'facebook',
      icon: '📘',
      url: `https://www.facebook.com/sharer/sharer.php?u=${encodeURIComponent(url)}`,
    },
    {
      name: 'twitter',
      icon: '🐦',
      url: `https://twitter.com/intent/tweet?text=${encodeURIComponent(title)}&url=${encodeURIComponent(url)}`,
    },
    {
      name: 'whatsapp',
      icon: '💬',
      url: `https://wa.me/?text=${encodeURIComponent(title + ' ' + url)}`,
    },
  ];

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  return (
    <div data-share-buttons className="share-buttons">
      <span className="share-label">{t('share.title')}</span>
      <div className="share-icons">
        {shareLinks.map((link) => (
          <motion.a
            key={link.name}
            href={link.url}
            target="_blank"
            rel="noopener noreferrer"
            className="share-icon"
            whileHover={{ scale: 1.1 }}
            whileTap={{ scale: 0.95 }}
            aria-label={`Share on ${link.name}`}
          >
            {link.icon}
          </motion.a>
        ))}
        <motion.button
          className="share-icon copy"
          onClick={copyLink}
          whileHover={{ scale: 1.1 }}
          whileTap={{ scale: 0.95 }}
          aria-label={t('share.copy')}
        >
          {copied ? '✅' : '🔗'}
        </motion.button>
      </div>
      <style>{`
        .share-buttons {
          display: flex;
          align-items: center;
          gap: 1rem;
        }

        .share-label {
          font-size: 0.8rem;
          text-transform: uppercase;
          letter-spacing: 0.1em;
          color: rgba(255, 255, 255, 0.6);
        }

        .share-icons {
          display: flex;
          gap: 0.5rem;
        }

        .share-icon {
          width: 36px;
          height: 36px;
          display: flex;
          align-items: center;
          justify-content: center;
          background: rgba(255, 255, 255, 0.1);
          border: 1px solid rgba(255, 255, 255, 0.2);
          border-radius: 50%;
          font-size: 1rem;
          cursor: pointer;
          text-decoration: none;
          transition: all 0.3s ease;
        }

        .share-icon:hover {
          background: rgba(255, 223, 0, 0.2);
          border-color: rgba(255, 223, 0, 0.5);
        }

        .share-icon.copy {
          background: transparent;
          border: none;
          font-size: 1.1rem;
        }
      `}</style>
    </div>
  );
}
