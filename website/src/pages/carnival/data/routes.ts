import type { CarnivalRoute } from '../types';

export const routes: CarnivalRoute[] = [
  {
    id: 'sambodromo',
    titleKey: 'routes.sambodromo.title',
    subtitleKey: 'routes.sambodromo.type',
    descriptionKey: 'routes.sambodromo.desc',
    type: 'night',
    color: '#ffdf00',
    stops: [
      { name: 'stops.praçaOnze', order: 1 },
      { name: 'stops.apoteose', order: 2 },
      { name: 'stops.dispersion', order: 3 },
    ],
    pathData: 'M40 220 C 140 140, 220 240, 360 150',
    gradientId: 'route1',
    heroImage: 'https://images.unsplash.com/photo-1551632436-cbf8dd35adfa?w=1600&q=80',
    gallery: [
      'https://images.unsplash.com/photo-1533174072545-7a4b6ad7a6c3?w=800&q=80',
      'https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=800&q=80',
      'https://images.unsplash.com/photo-1548115184-bc6544d06a58?w=800&q=80',
      'https://images.unsplash.com/photo-1509281373149-e957c6296406?w=800&q=80',
    ],
    tips: [
      'tips.sambodromo.arrive',
      'tips.sambodromo.seat',
      'tips.sambodromo.camera',
    ],
    itinerary: [
      { time: '18:00', titleKey: 'itinerary.sambodromo.arrival', descriptionKey: 'itinerary.sambodromo.arrival.desc' },
      { time: '21:00', titleKey: 'itinerary.sambodromo.start', descriptionKey: 'itinerary.sambodromo.start.desc' },
      { time: '23:30', titleKey: 'itinerary.sambodromo.apoteose', descriptionKey: 'itinerary.sambodromo.apoteose.desc' },
      { time: '01:00', titleKey: 'itinerary.sambodromo.end', descriptionKey: 'itinerary.sambodromo.end.desc' },
    ],
  },
  {
    id: 'centro',
    titleKey: 'routes.centro.title',
    subtitleKey: 'routes.centro.type',
    descriptionKey: 'routes.centro.desc',
    type: 'urban',
    color: '#00b36b',
    stops: [
      { name: 'stops.cinelândia', order: 1 },
      { name: 'stops.lapa', order: 2 },
      { name: 'stops.praçaXV', order: 3 },
    ],
    pathData: 'M60 70 C 150 110, 230 40, 360 80',
    gradientId: 'route2',
    heroImage: 'https://images.unsplash.com/photo-1483729558449-99ef09a8c325?w=1600&q=80',
    gallery: [
      'https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=800&q=80',
      'https://images.unsplash.com/photo-1544531586-fde5298cdd40?w=800&q=80',
      'https://images.unsplash.com/photo-1596395819057-79db931666d1?w=800&q=80',
      'https://images.unsplash.com/photo-1562654501-a0ccc0fc3fb1?w=800&q=80',
    ],
    tips: [
      'tips.centro.walk',
      'tips.centro.music',
      'tips.centro.food',
    ],
    itinerary: [
      { time: '14:00', titleKey: 'itinerary.centro.cinelandia', descriptionKey: 'itinerary.centro.cinelandia.desc' },
      { time: '16:00', titleKey: 'itinerary.centro.lapa', descriptionKey: 'itinerary.centro.lapa.desc' },
      { time: '19:00', titleKey: 'itinerary.centro.praca', descriptionKey: 'itinerary.centro.praca.desc' },
      { time: '22:00', titleKey: 'itinerary.centro.night', descriptionKey: 'itinerary.centro.night.desc' },
    ],
  },
  {
    id: 'orla',
    titleKey: 'routes.orla.title',
    subtitleKey: 'routes.orla.type',
    descriptionKey: 'routes.orla.desc',
    type: 'dawn',
    color: '#0084c6',
    stops: [
      { name: 'stops.copacabana', order: 1 },
      { name: 'stops.ipanema', order: 2 },
      { name: 'stops.leblon', order: 3 },
    ],
    pathData: 'M50 150 C 140 190, 210 120, 360 210',
    gradientId: 'route3',
    heroImage: 'https://images.unsplash.com/photo-1582418682369-6a8cc6872880?w=1600&q=80',
    gallery: [
      'https://images.unsplash.com/photo-1562654501-a0ccc0fc3fb1?w=800&q=80',
      'https://images.unsplash.com/photo-1596395819057-79db931666d1?w=800&q=80',
      'https://images.unsplash.com/photo-1483729558449-99ef09a8c325?w=800&q=80',
      'https://images.unsplash.com/photo-1582418682369-6a8cc6872880?w=800&q=80',
    ],
    tips: [
      'tips.orla.sunrise',
      'tips.orla.beach',
      'tips.orla.breakfast',
    ],
    itinerary: [
      { time: '04:00', titleKey: 'itinerary.orla.copa', descriptionKey: 'itinerary.orla.copa.desc' },
      { time: '06:00', titleKey: 'itinerary.orla.sunrise', descriptionKey: 'itinerary.orla.sunrise.desc' },
      { time: '08:00', titleKey: 'itinerary.orla.ipanema', descriptionKey: 'itinerary.orla.ipanema.desc' },
      { time: '10:00', titleKey: 'itinerary.orla.leblon', descriptionKey: 'itinerary.orla.leblon.desc' },
    ],
  },
];

export const homePageImages = {
  heroMain: 'https://images.unsplash.com/photo-1551632436-cbf8dd35adfa?w=1600&q=80',
  heroLeft: 'https://images.unsplash.com/photo-1533174072545-7a4b6ad7a6c3?w=900&q=80',
  heroRight: 'https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=900&q=80',
};

export const galleryImages = [
  'https://images.unsplash.com/photo-1548115184-bc6544d06a58?w=900&q=80',
  'https://images.unsplash.com/photo-1509281373149-e957c6296406?w=900&q=80',
  'https://images.unsplash.com/photo-1544531586-fde5298cdd40?w=900&q=80',
  'https://images.unsplash.com/photo-1596395819057-79db931666d1?w=900&q=80',
  'https://images.unsplash.com/photo-1582418682369-6a8cc6872880?w=900&q=80',
  'https://images.unsplash.com/photo-1562654501-a0ccc0fc3fb1?w=900&q=80',
];
