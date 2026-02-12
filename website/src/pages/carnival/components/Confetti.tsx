import { motion } from 'framer-motion';
import { useConfetti } from '../hooks/useConfetti';

export function Confetti() {
  const particles = useConfetti(40);

  return (
    <div data-confetti-container className="confetti-container">
      {particles.map((particle) => (
        <motion.div
          key={particle.id}
          className="confetti-piece"
          initial={{ y: -20, x: `${particle.x}vw`, opacity: 1, rotate: 0 }}
          animate={{
            y: '110vh',
            rotate: 720,
            opacity: 0,
          }}
          transition={{
            duration: particle.duration,
            delay: particle.delay,
            ease: 'linear',
            repeat: Infinity,
          }}
          style={{
            background: particle.color,
            width: particle.size,
            height: particle.size,
            borderRadius: particle.borderRadius,
          }}
        />
      ))}
      <style>{`
        .confetti-container {
          position: fixed;
          inset: 0;
          pointer-events: none;
          z-index: 0;
          overflow: hidden;
        }

        .confetti-piece {
          position: absolute;
          top: 0;
        }
      `}</style>
    </div>
  );
}
