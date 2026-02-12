import { useEffect, useState } from 'react';

interface Particle {
  id: number;
  x: number;
  delay: number;
  duration: number;
  color: string;
  size: number;
  borderRadius: string;
}

const COLORS = ['#ffdf00', '#009c3b', '#ff6b3d', '#ffffff', '#0084c6'];

export function useConfetti(count: number = 40) {
  const [particles, setParticles] = useState<Particle[]>([]);

  useEffect(() => {
    const newParticles: Particle[] = Array.from({ length: count }, (_, i) => ({
      id: i,
      x: Math.random() * 100,
      delay: Math.random() * 5,
      duration: Math.random() * 3 + 3,
      color: COLORS[Math.floor(Math.random() * COLORS.length)],
      size: Math.random() * 4 + 4,
      borderRadius: Math.random() > 0.5 ? '50%' : '2px',
    }));
    setParticles(newParticles);
  }, [count]);

  return particles;
}
