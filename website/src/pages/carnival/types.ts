export interface CarnivalRoute {
  id: string;
  titleKey: string;
  subtitleKey: string;
  descriptionKey: string;
  type: 'night' | 'urban' | 'dawn';
  color: string;
  stops: RouteStop[];
  pathData: string;
  gradientId: string;
  heroImage: string;
  gallery: string[];
  tips: string[];
  itinerary: ItineraryItem[];
}

export interface ItineraryItem {
  time: string;
  titleKey: string;
  descriptionKey: string;
}

export interface RouteStop {
  name: string;
  order: number;
}

export interface ScheduleEvent {
  id: string;
  titleKey: string;
  descriptionKey: string;
  location: string;
  date: string;
  time: string;
  category: 'desfile' | 'bloco' | 'baile' | 'cultural';
  price?: string;
  isLive?: boolean;
}

export interface TravelTip {
  id: string;
  titleKey: string;
  items: string[];
}

export interface GalleryImage {
  id: string;
  src: string;
  altKey: string;
  captionKey: string;
  category: 'samba' | 'colors' | 'crowd' | 'sunset' | 'beach' | 'city' | 'dancers' | 'costumes' | 'landmarks';
}

export interface Language {
  code: 'pt' | 'es';
  label: string;
}

export interface WeatherData {
  temperature: number;
  condition: string;
  humidity: number;
  icon: string;
}

export interface CountdownData {
  days: number;
  hours: number;
  minutes: number;
  seconds: number;
}

export type TranslationKey = string;

export interface Translations {
  [key: string]: {
    pt: string;
    es: string;
  };
}
