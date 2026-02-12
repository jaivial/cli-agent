import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'

const galleryImages = [
  { url: 'https://st4.depositphotos.com/13193636/24686/i/450/depositphotos_246868916-stock-photo-cute-cat-looking-at-camera.jpg', caption: 'Curious Kitty' },
  { url: 'https://st4.depositphotos.com/15933568/23946/i/450/depositphotos_239468074-stock-photo-adorable-kitten.jpg', caption: 'Adorable Kitten' },
  { url: 'https://st4.depositphotos.com/1010612/28447/i/450/depositphotos_284475630-stock-photo-cat-playing.jpg', caption: 'Playful Cat' },
  { url: 'https://st4.depositphotos.com/27350638/38967/i/450/depositphotos_389677078-stock-photo-sleeping-cat.jpg', caption: 'Sleepy Head' },
  { url: 'https://st4.depositphotos.com/1688730/5085/i/450/depositphotos_50856125-stock-photo-jumping-cat.jpg', caption: 'Acrobat Cat' },
  { url: 'https://st4.depositphotos.com/13193636/25188/i/450/depositphotos_251883376-stock-photo-cat-with-sunglasses.jpg', caption: 'Cool Cat' },
  { url: 'https://st4.depositphotos.com/13356592/23683/i/450/depositphotos_236839590-stock-photo-happy-cat.jpg', caption: 'Happy Smile' },
  { url: 'https://st4.depositphotos.com/27350638/38964/i/450/depositphotos_389648842-stock-photo-cat-in-box.jpg', caption: 'Box Lover' },
  { url: 'https://st4.depositphotos.com/1010612/28533/i/450/depositphotos_285336944-stock-photo-cat-with-toy.jpg', caption: 'Toy Hunter' },
  { url: 'https://st4.depositphotos.com/13193636/31868/i/450/depositphotos_318686488-stock-photo-cat-stretching.jpg', caption: 'Morning Stretch' },
  { url: 'https://st4.depositphotos.com/1688730/4865/i/450/depositphotos_48652845-stock-photo-cat-looking-out-window.jpg', caption: 'Window Watcher' },
  { url: 'https://st4.depositphotos.com/15933568/23948/i/450/depositphotos_239487062-stock-photo-grooming-cat.jpg', caption: 'Self Care' },
]

const Gallery = () => {
  const [selectedImage, setSelectedImage] = useState(null)

  return (
    <motion.div 
      className="page-container"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      transition={{ duration: 0.5 }}
    >
      <h1 className="page-title">Cat Gallery 📸</h1>
      <p style={{ textAlign: 'center', marginBottom: '2rem', fontSize: '1.1rem' }}>
        Click on any image to see it in full size!
      </p>
      
      <div className="grid-3">
        {galleryImages.map((image, index) => (
          <motion.div 
            key={index}
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: index * 0.05, duration: 0.4 }}
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            style={{ cursor: 'pointer' }}
            onClick={() => setSelectedImage(image)}
          >
            <img 
              src={image.url} 
              alt={image.caption}
              className="gallery-image"
            />
            <p style={{ textAlign: 'center', marginTop: '0.5rem', color: '#666', fontWeight: '500' }}>
              {image.caption}
            </p>
          </motion.div>
        ))}
      </div>
      
      <AnimatePresence>
        {selectedImage && (
          <motion.div 
            className="modal"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={() => setSelectedImage(null)}
            style={{
              position: 'fixed',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              background: 'rgba(0, 0, 0, 0.9)',
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              zIndex: 2000,
              cursor: 'pointer'
            }}
          >
            <motion.div 
              initial={{ scale: 0.8 }}
              animate={{ scale: 1 }}
              exit={{ scale: 0.8 }}
              onClick={(e) => e.stopPropagation()}
            >
              <img 
                src={selectedImage.url} 
                alt={selectedImage.caption}
                style={{ 
                  maxWidth: '90vw', 
                  maxHeight: '90vh', 
                  borderRadius: '10px' 
                }}
              />
              <p style={{ 
                color: 'white', 
                textAlign: 'center', 
                marginTop: '1rem', 
                fontSize: '1.2rem' 
              }}>
                {selectedImage.caption}
              </p>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}

export default Gallery
