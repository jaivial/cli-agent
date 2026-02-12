import { useEffect, useRef } from 'react';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';

interface InteractiveMapProps {
  center?: [number, number];
  zoom?: number;
  markers?: Array<{
    position: [number, number];
    title: string;
    color?: string;
  }>;
}

export function InteractiveMap({ 
  center = [-22.9068, -43.1729], 
  zoom = 13,
  markers = []
}: InteractiveMapProps) {
  const mapRef = useRef<HTMLDivElement>(null);
  const { language } = useLanguage();
  const t = (key: string) => translations[key]?.[language] || key;

  useEffect(() => {
    if (!mapRef.current) return;

    const mapContainer = mapRef.current;
    
    const mapStyles: React.CSSProperties = {
      width: '100%',
      height: '400px',
      borderRadius: '2rem',
      background: 'linear-gradient(135deg, rgba(6, 15, 37, 0.9), rgba(0, 132, 198, 0.3))',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: '1rem',
      position: 'relative' as const,
      overflow: 'hidden',
    };

    Object.assign(mapContainer.style, mapStyles);

    const pattern = document.createElement('div');
    pattern.style.cssText = `
      position: absolute;
      inset: 0;
      background-image: 
        radial-gradient(circle at 20% 30%, rgba(255, 223, 0, 0.15) 0%, transparent 50%),
        radial-gradient(circle at 80% 70%, rgba(255, 107, 61, 0.15) 0%, transparent 50%);
    `;
    mapContainer.appendChild(pattern);

    const title = document.createElement('h4');
    title.textContent = t('map.title');
    title.style.cssText = `
      font-family: 'Fraunces', serif;
      font-size: 1.5rem;
      color: #ffdf00;
      margin: 0;
      z-index: 1;
    `;
    mapContainer.appendChild(title);

    const subtitle = document.createElement('p');
    subtitle.textContent = t('map.zoom');
    subtitle.style.cssText = `
      color: rgba(255, 255, 255, 0.6);
      font-size: 0.9rem;
      margin: 0;
      z-index: 1;
    `;
    mapContainer.appendChild(subtitle);

    const defaultMarkers = [
      { position: [-22.9118, -43.2093] as [number, number], title: 'Sambódromo', color: '#ffdf00' },
      { position: [-22.9070, -43.1888] as [number, number], title: 'Lapa', color: '#00b36b' },
      { position: [-22.9711, -43.1822] as [number, number], title: 'Copacabana', color: '#0084c6' },
      { position: [-22.9838, -43.2096] as [number, number], title: 'Ipanema', color: '#0084c6' },
      { position: [-22.9904, -43.2156] as [number, number], title: 'Leblon', color: '#0084c6' },
    ];

    const allMarkers = markers.length > 0 ? markers : defaultMarkers;

    allMarkers.forEach((marker, index) => {
      const markerEl = document.createElement('div');
      markerEl.style.cssText = `
        position: absolute;
        left: ${30 + (index * 12)}%;
        top: ${40 + (index % 3) * 15}%;
        width: 16px;
        height: 16px;
        border-radius: 50%;
        background: ${marker.color || '#ffdf00'};
        box-shadow: 0 0 15px ${marker.color || '#ffdf00'};
        cursor: pointer;
        transition: transform 0.3s ease;
        z-index: 1;
      `;
      markerEl.title = marker.title;
      markerEl.onmouseenter = () => {
        markerEl.style.transform = 'scale(1.3)';
      };
      markerEl.onmouseleave = () => {
        markerEl.style.transform = 'scale(1)';
      };
      mapContainer.appendChild(markerEl);
    });

    return () => {
      while (mapContainer.firstChild) {
        mapContainer.removeChild(mapContainer.firstChild);
      }
    };
  }, [center, zoom, markers, language, t]);

  return <div ref={mapRef} data-interactive-map />;
}
