import { motion } from 'framer-motion';

interface FloatingParticlesProps {
  count?: number;
}

const warmColors = ['#ffdf00', '#ff6b3d', '#ff00ff', '#8a2be2'];

export function FloatingParticles({ count = 30 }: FloatingParticlesProps) {
  const particles = Array.from({ length: count }, (_, i) => ({
    id: i,
    x: Math.random() * 100,
    delay: Math.random() * 10,
    duration: 10 + Math.random() * 10,
    size: 2 + Math.random() * 4,
    opacity: 0.3 + Math.random() * 0.5,
    color: warmColors[Math.floor(Math.random() * warmColors.length)],
  }));

  return (
    <div data-floating-particles className="floating-particles">
      {particles.map((p) => (
        <motion.div
          key={p.id}
          className="particle"
          initial={{ 
            x: `${p.x}vw`, 
            y: '110vh', 
            opacity: 0 
          }}
          animate={{ 
            y: '-10vh',
            opacity: [0, p.opacity, p.opacity, 0],
          }}
          transition={{ 
            duration: p.duration, 
            delay: p.delay, 
            repeat: Infinity,
            ease: 'linear',
          }}
          style={{
            width: p.size,
            height: p.size,
            left: `${p.x}%`,
            background: p.color,
            boxShadow: `0 0 10px ${p.color}, 0 0 20px ${p.color}50`,
          }}
        />
      ))}
      <style>{`
        .floating-particles {
          position: fixed;
          inset: 0;
          pointer-events: none;
          z-index: 0;
          overflow: hidden;
        }

        .particle {
          position: absolute;
          bottom: 0;
          border-radius: 50%;
        }
      `}</style>
    </div>
  );
}
