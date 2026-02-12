import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { useScrollProgress } from '../hooks/useScrollProgress';
import { translations } from '../translations';
import { FloatingParticles } from './FloatingParticles';

interface CarnivalLayoutProps {
  children: React.ReactNode;
}

export function CarnivalLayout({ children }: CarnivalLayoutProps) {
  const { language, toggleLanguage } = useLanguage();
  const location = useLocation();
  const scrollProgress = useScrollProgress();
  const [mobileMenuOpen, setMobileMenuOpen] = React.useState(false);

  const t = (key: string) => translations[key]?.[language] || key;

  const navItems = [
    { path: '/carnival', label: t('nav.home'), end: true },
    { path: '/carnival/sambodromo', label: t('nav.sambodromo') },
    { path: '/carnival/centro-lapa', label: t('nav.centrolapa') },
    { path: '/carnival/orla', label: t('nav.orla') },
    { path: '/carnival/schedule', label: t('nav.schedule') },
    { path: '/carnival/tips', label: t('nav.tips') },
  ];

  const isActive = (path: string, end?: boolean) => {
    if (end) return location.pathname === path;
    return location.pathname.startsWith(path);
  };

  return (
    <div data-carnival-layout className="carnival-layout">
      <FloatingParticles count={25} />
      
      <div className="carnival-bg">
        <div className="carnival-bg-gradient" />
        <div className="carnival-bg-pattern" />
        <div className="glowing-orb orb-gold" />
        <div className="glowing-orb orb-orange" />
        <div className="glowing-orb orb-magenta" />
        <div className="glowing-orb orb-purple" />
        <div className="carnival-bg-glow-1" />
        <div className="carnival-bg-glow-2" />
        <div className="carnival-bg-glow-3" />
      </div>

      <motion.header
        className="carnival-topbar"
        initial={{ y: -100 }}
        animate={{ y: 0 }}
        transition={{ type: 'spring', stiffness: 100, damping: 20 }}
      >
        <div className="carnival-topbar-inner">
          <Link to="/carnival" className="carnival-brand">
            <span>🎭</span> Carnaval Rio
          </Link>

          <button
            className="carnival-menu-toggle"
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            aria-expanded={mobileMenuOpen}
            aria-label="Menu"
          >
            <span className={`hamburger ${mobileMenuOpen ? 'open' : ''}`} />
          </button>

          <nav className={`carnival-nav ${mobileMenuOpen ? 'open' : ''}`}>
            {navItems.map((item) => (
              <Link
                key={item.path}
                to={item.path}
                className={isActive(item.path, item.end) ? 'active' : ''}
                onClick={() => setMobileMenuOpen(false)}
              >
                {item.label}
              </Link>
            ))}
          </nav>

          <div className="carnival-topbar-actions">
            <div className="carnival-lang-toggle" role="group" aria-label="Idioma">
              <button
                className={language === 'pt' ? 'active' : ''}
                onClick={() => language !== 'pt' && toggleLanguage()}
                aria-pressed={language === 'pt'}
              >
                PT
              </button>
              <button
                className={language === 'es' ? 'active' : ''}
                onClick={() => language !== 'es' && toggleLanguage()}
                aria-pressed={language === 'es'}
              >
                ES
              </button>
            </div>
          </div>
        </div>

        <motion.div
          className="carnival-scroll-progress"
          style={{ scaleX: scrollProgress / 100 }}
        />
      </motion.header>

      <AnimatePresence mode="wait">
        <motion.main
          key={location.pathname}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -20 }}
          transition={{ duration: 0.3 }}
          className="carnival-main"
        >
          {children}
        </motion.main>
      </AnimatePresence>

      <footer className="carnival-footer">
        <div className="carnival-footer-links">
          <span>{t('footer.contact')}</span>
          <span>{t('footer.follow')}</span>
        </div>
        <p>{t('footer.copyright')}</p>
      </footer>

      <style>{`
        .carnival-layout {
          min-height: 100vh;
          position: relative;
        }

        .carnival-bg {
          position: fixed;
          inset: 0;
          z-index: -2;
          pointer-events: none;
        }

        .carnival-bg-gradient {
          position: absolute;
          inset: 0;
          background: linear-gradient(135deg, #1a0533 0%, #2d1b4e 25%, #4a1942 50%, #6b2d5c 75%, #ff6b3d 100%);
        }

        .carnival-bg-pattern {
          position: absolute;
          inset: 0;
          background-image: 
            radial-gradient(circle at 20% 80%, rgba(255, 0, 128, 0.08) 0%, transparent 40%),
            radial-gradient(circle at 80% 20%, rgba(255, 165, 0, 0.08) 0%, transparent 40%),
            radial-gradient(circle at 40% 40%, rgba(138, 43, 226, 0.06) 0%, transparent 30%);
          animation: patternFloat 20s ease-in-out infinite;
        }

        @keyframes patternFloat {
          0%, 100% { transform: translate(0, 0); }
          50% { transform: translate(-10px, 10px); }
        }

        .carnival-bg-glow-1 {
          position: absolute;
          width: 600px;
          height: 600px;
          border-radius: 50%;
          top: -200px;
          right: -100px;
          background: radial-gradient(circle, rgba(255, 215, 0, 0.2) 0%, rgba(255, 107, 61, 0.1) 40%, transparent 70%);
          filter: blur(40px);
          animation: glowPulse 8s ease-in-out infinite;
        }

        .carnival-bg-glow-2 {
          position: absolute;
          width: 500px;
          height: 500px;
          border-radius: 50%;
          bottom: -150px;
          left: -100px;
          background: radial-gradient(circle, rgba(255, 0, 128, 0.15) 0%, rgba(138, 43, 226, 0.08) 40%, transparent 70%);
          filter: blur(40px);
          animation: glowPulse 10s ease-in-out infinite reverse;
        }

        .carnival-bg-glow-3 {
          position: absolute;
          width: 400px;
          height: 400px;
          border-radius: 50%;
          top: 50%;
          left: 50%;
          transform: translate(-50%, -50%);
          background: radial-gradient(circle, rgba(0, 132, 198, 0.1) 0%, transparent 60%);
          filter: blur(30px);
          animation: glowPulse 12s ease-in-out infinite;
        }

        .glowing-orb {
          position: absolute;
          border-radius: 50%;
          filter: blur(40px);
          animation: orbPulse 6s ease-in-out infinite;
        }

        .orb-gold {
          width: 300px;
          height: 300px;
          top: 10%;
          left: 5%;
          background: radial-gradient(circle, rgba(255, 215, 0, 0.4) 0%, rgba(255, 215, 0, 0.1) 40%, transparent 70%);
          animation-delay: 0s;
        }

        .orb-orange {
          width: 250px;
          height: 250px;
          top: 60%;
          right: 10%;
          background: radial-gradient(circle, rgba(255, 107, 61, 0.4) 0%, rgba(255, 107, 61, 0.1) 40%, transparent 70%);
          animation-delay: 2s;
        }

        .orb-magenta {
          width: 200px;
          height: 200px;
          bottom: 20%;
          left: 20%;
          background: radial-gradient(circle, rgba(255, 0, 128, 0.35) 0%, rgba(255, 0, 128, 0.1) 40%, transparent 70%);
          animation-delay: 4s;
        }

        .orb-purple {
          width: 280px;
          height: 280px;
          top: 30%;
          right: 25%;
          background: radial-gradient(circle, rgba(138, 43, 226, 0.3) 0%, rgba(138, 43, 226, 0.1) 40%, transparent 70%);
          animation-delay: 1s;
        }

        @keyframes orbPulse {
          0%, 100% { opacity: 0.5; transform: scale(1); }
          50% { opacity: 0.8; transform: scale(1.15);
        }

        @keyframes glowPulse {
          0%, 100% { opacity: 0.6; transform: scale(1); }
          50% { opacity: 0.8; transform: scale(1.05); }
        }

        .carnival-topbar {
          position: sticky;
          top: 0;
          z-index: 200;
          backdrop-filter: blur(16px);
          background: rgba(6, 13, 31, 0.85);
          border-bottom: 1px solid rgba(255, 255, 255, 0.08);
        }

        .carnival-scroll-progress {
          position: absolute;
          bottom: 0;
          left: 0;
          width: 100%;
          height: 3px;
          background: linear-gradient(90deg, #ff00ff, #ffdf00, #ff6b3d);
          transform-origin: 0%;
        }

        .carnival-topbar-inner {
          max-width: 1400px;
          margin: 0 auto;
          padding: 1rem 2rem;
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 1.5rem;
        }

        .carnival-brand {
          font-family: 'Fraunces', serif;
          font-size: 1.5rem;
          font-weight: 700;
          letter-spacing: 0.02em;
          color: #ffdf00;
          display: flex;
          align-items: center;
          gap: 0.5rem;
        }

        .carnival-nav {
          display: flex;
          gap: 1rem;
          font-size: 0.85rem;
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.08em;
        }

        .carnival-nav a {
          position: relative;
          padding: 0.5rem 0.8rem;
          border-radius: 1.5rem;
          transition: all 0.3s ease;
        }

        .carnival-nav a::after {
          content: '';
          position: absolute;
          left: 50%;
          bottom: 0;
          width: 0;
          height: 2px;
          background: linear-gradient(90deg, #ff00ff, #ffdf00);
          transition: all 0.3s ease;
          transform: translateX(-50%);
        }

        .carnival-nav a.active,
        .carnival-nav a:hover {
          color: #ffdf00;
          background: rgba(255, 223, 0, 0.1);
        }

        .carnival-nav a.active::after,
        .carnival-nav a:hover::after {
          width: 60%;
        }

        .carnival-topbar-actions {
          display: flex;
          align-items: center;
          gap: 0.8rem;
        }

        .carnival-lang-toggle {
          display: flex;
          gap: 0.3rem;
          padding: 0.2rem;
          background: rgba(255, 255, 255, 0.08);
          border-radius: 999px;
        }

        .carnival-lang-toggle button {
          border: none;
          background: transparent;
          color: #f6f1e5;
          font-weight: 700;
          padding: 0.35rem 0.9rem;
          border-radius: 999px;
          cursor: pointer;
          font-size: 0.85rem;
          transition: all 0.3s ease;
        }

        .carnival-lang-toggle button.active {
          background: linear-gradient(135deg, #ff00ff, #ff6b3d);
          color: white;
        }

        .carnival-menu-toggle {
          display: none;
          border: 1px solid rgba(255, 255, 255, 0.2);
          background: transparent;
          color: #f6f1e5;
          padding: 0.5rem 0.8rem;
          border-radius: 8px;
          font-size: 0.9rem;
          cursor: pointer;
        }

        .hamburger {
          display: block;
          width: 20px;
          height: 2px;
          background: #f6f1e5;
          position: relative;
          transition: all 0.3s ease;
        }

        .hamburger::before,
        .hamburger::after {
          content: '';
          position: absolute;
          width: 20px;
          height: 2px;
          background: #f6f1e5;
          transition: all 0.3s ease;
        }

        .hamburger::before { top: -6px; }
        .hamburger::after { top: 6px; }

        .hamburger.open {
          background: transparent;
        }

        .hamburger.open::before {
          top: 0;
          transform: rotate(45deg);
        }

        .hamburger.open::after {
          top: 0;
          transform: rotate(-45deg);
        }

        .carnival-main {
          max-width: 1400px;
          margin: 0 auto;
          padding: 0 2rem 5rem;
        }

        .carnival-footer {
          border-top: 1px solid rgba(255, 255, 255, 0.08);
          padding: 2.5rem 0 3.5rem;
          text-align: center;
          color: rgba(255, 255, 255, 0.6);
        }

        .carnival-footer-links {
          display: flex;
          justify-content: center;
          flex-wrap: wrap;
          gap: 1.2rem;
          margin-bottom: 1rem;
        }

        @media (max-width: 1100px) {
          .carnival-topbar-inner {
            flex-wrap: wrap;
          }

          .carnival-menu-toggle {
            display: inline-flex;
            align-items: center;
            justify-content: center;
          }

          .carnival-nav,
          .carnival-topbar-actions {
            width: 100%;
            flex-direction: column;
            align-items: flex-start;
            display: none;
          }

          .carnival-nav.open,
          .carnival-topbar-actions.open {
            display: flex;
          }

          .carnival-nav {
            padding: 1rem 0;
          }

          .carnival-topbar-actions {
            padding-bottom: 1rem;
          }
        }
      `}</style>
    </div>
  );
}
