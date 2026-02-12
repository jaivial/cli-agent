import { useState, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useLanguage } from '../hooks/useLanguage';
import { translations } from '../translations';
import { galleryImages } from '../data/gallery';
import type { GalleryImage } from '../types';

type Category = 'all' | GalleryImage['category'];

const categories: { key: Category; labelKey: string }[] = [
  { key: 'all', labelKey: 'gallery.filter.all' },
  { key: 'samba', labelKey: 'gallery.samba' },
  { key: 'colors', labelKey: 'gallery.colors' },
  { key: 'crowd', labelKey: 'gallery.crowd' },
  { key: 'sunset', labelKey: 'gallery.sunset' },
  { key: 'beach', labelKey: 'gallery.beach' },
  { key: 'city', labelKey: 'gallery.city' },
  { key: 'dancers', labelKey: 'gallery.dancers' },
  { key: 'costumes', labelKey: 'gallery.costumes' },
  { key: 'landmarks', labelKey: 'gallery.landmarks' },
];

export function GallerySection() {
  const { language } = useLanguage();
  const [selectedCategory, setSelectedCategory] = useState<Category>('all');
  const [selectedImage, setSelectedImage] = useState<string | null>(null);
  const t = (key: string) => translations[key]?.[language] || key;

  const filteredImages = useMemo(() => {
    if (selectedCategory === 'all') return galleryImages;
    return galleryImages.filter((img) => img.category === selectedCategory);
  }, [selectedCategory]);

  return (
    <section data-gallery-section>
      <motion.h2
        className="carnival-section-title"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5 }}
      >
        {t('gallery.title')}
      </motion.h2>
      <motion.p
        className="carnival-section-subtitle"
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5, delay: 0.1 }}
      >
        {t('gallery.subtitle')}
      </motion.p>

      <motion.div 
        className="gallery-filters"
        initial={{ opacity: 0, y: 10 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.4, delay: 0.2 }}
      >
        {categories.map((cat) => (
          <button
            key={cat.key}
            className={`gallery-filter-btn ${selectedCategory === cat.key ? 'active' : ''}`}
            onClick={() => setSelectedCategory(cat.key)}
          >
            {t(cat.labelKey)}
          </button>
        ))}
      </motion.div>

      <motion.div 
        className="carnival-gallery"
        layout
      >
        <AnimatePresence mode="popLayout">
          {filteredImages.map((image, index) => (
            <motion.figure
              key={image.id}
              className="carnival-gallery-item"
              layout
              initial={{ opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.9 }}
              viewport={{ once: true }}
              transition={{ duration: 0.4, delay: index * 0.05 }}
              whileHover={{ scale: 1.03 }}
              onClick={() => setSelectedImage(image.src)}
            >
              <img src={image.src} alt={t(image.altKey)} />
              <motion.div
                className="carnival-gallery-overlay"
                initial={{ opacity: 0 }}
                whileHover={{ opacity: 1 }}
              >
                <span>{t(image.captionKey)}</span>
              </motion.div>
            </motion.figure>
          ))}
        </AnimatePresence>
      </motion.div>

      <AnimatePresence>
        {selectedImage && (
          <motion.div
            className="carnival-lightbox"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={() => setSelectedImage(null)}
          >
            <motion.div
              className="carnival-lightbox-content"
              initial={{ scale: 0.8 }}
              animate={{ scale: 1 }}
              exit={{ scale: 0.8 }}
              onClick={(e) => e.stopPropagation()}
            >
              <button
                className="carnival-lightbox-close"
                onClick={() => setSelectedImage(null)}
              >
                ✕
              </button>
              <img src={selectedImage} alt="Gallery" />
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      <style>{`
        .gallery-filters {
          display: flex;
          flex-wrap: wrap;
          gap: 0.6rem;
          margin-bottom: 2rem;
        }

        .gallery-filter-btn {
          padding: 0.5rem 1rem;
          background: rgba(255, 255, 255, 0.08);
          border: 1px solid rgba(255, 255, 255, 0.1);
          border-radius: 999px;
          color: rgba(255, 255, 255, 0.7);
          font-size: 0.85rem;
          font-weight: 600;
          cursor: pointer;
          transition: all 0.3s ease;
        }

        .gallery-filter-btn:hover {
          background: rgba(255, 255, 255, 0.15);
        }

        .gallery-filter-btn.active {
          background: #ffdf00;
          color: #060f25;
          border-color: #ffdf00;
        }

        .carnival-gallery {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
          gap: 1.2rem;
        }

        .carnival-gallery-item {
          position: relative;
          border-radius: 2rem;
          overflow: hidden;
          aspect-ratio: 4 / 3;
          cursor: pointer;
          margin: 0;
        }

        .carnival-gallery-item img {
          width: 100%;
          height: 100%;
          object-fit: cover;
          transition: transform 0.5s ease;
        }

        .carnival-gallery-overlay {
          position: absolute;
          inset: 0;
          background: rgba(6, 15, 37, 0.6);
          display: flex;
          align-items: center;
          justify-content: center;
          opacity: 0;
          transition: opacity 0.3s ease;
        }

        .carnival-gallery-overlay span {
          background: rgba(6, 15, 37, 0.8);
          padding: 0.5rem 1rem;
          border-radius: 999px;
          font-size: 0.85rem;
        }

        .carnival-lightbox {
          position: fixed;
          inset: 0;
          background: rgba(6, 13, 31, 0.95);
          z-index: 1000;
          display: flex;
          align-items: center;
          justify-content: center;
          padding: 2rem;
          backdrop-filter: blur(10px);
        }

        .carnival-lightbox-content {
          position: relative;
          max-width: 90vw;
          max-height: 90vh;
        }

        .carnival-lightbox-content img {
          max-width: 100%;
          max-height: 90vh;
          border-radius: 2rem;
          object-fit: contain;
        }

        .carnival-lightbox-close {
          position: absolute;
          top: -40px;
          right: 0;
          background: rgba(255, 255, 255, 0.1);
          border: none;
          color: white;
          width: 36px;
          height: 36px;
          border-radius: 50%;
          font-size: 1.2rem;
          cursor: pointer;
          transition: background 0.3s ease;
        }

        .carnival-lightbox-close:hover {
          background: rgba(255, 255, 255, 0.2);
        }
      `}</style>
    </section>
  );
}
