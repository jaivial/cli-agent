import React from 'react'
import { motion } from 'framer-motion'

const About = () => {
  return (
    <motion.div 
      className="page-container"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      transition={{ duration: 0.5 }}
    >
      <h1 className="page-title">About Cat World ℹ️</h1>
      
      <motion.div 
        className="card"
        style={{ marginBottom: '2rem' }}
        initial={{ opacity: 0, x: -30 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ delay: 0.2 }}
      >
        <img 
          src="https://st4.depositphotos.com/27350638/38960/i/450/depositphotos_389603378-stock-photo-group-of-cats.jpg"
          alt="Group of cats"
          style={{ width: '100%', height: '300px', objectFit: 'cover', borderRadius: '10px', marginBottom: '1rem' }}
        />
        <h2 style={{ color: '#667eea', marginBottom: '1rem' }}>Our Mission</h2>
        <p style={{ lineHeight: '1.8', color: '#666' }}>
          Cat World is dedicated to celebrating and sharing knowledge about our feline friends. 
          We believe that every cat deserves love, care, and understanding. Our goal is to provide 
          valuable information about cat breeds, care tips, and showcase the beauty of cats through 
          our gallery.
        </p>
      </motion.div>
      
      <div className="grid-2">
        <motion.div 
          className="card"
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
        >
          <h3 style={{ color: '#667eea', marginBottom: '1rem' }}>Why We Love Cats</h3>
          <ul style={{ listStyle: 'none', lineHeight: '2', color: '#666' }}>
            <li>🐱 Their independent nature</li>
            <li>💤 The joy of watching them sleep</li>
            <li>🎾 Their playful antics</li>
            <li>💕 Their affectionate purring</li>
            <li>🧠 Their intelligence and curiosity</li>
          </ul>
        </motion.div>
        
        <motion.div 
          className="card"
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
        >
          <h3 style={{ color: '#667eea', marginBottom: '1rem' }}>What We Offer</h3>
          <ul style={{ listStyle: 'none', lineHeight: '2', color: '#666' }}>
            <li>📚 Information on 100+ cat breeds</li>
            <li>💡 Expert care tips and advice</li>
            <li>📸 Curated photo galleries</li>
            <li>🏠 Adoption resources</li>
            <li>🎓 Educational content</li>
          </ul>
        </motion.div>
      </div>
      
      <motion.div 
        className="card"
        style={{ marginTop: '2rem', background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', color: 'white' }}
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ delay: 0.5 }}
      >
        <h3 style={{ marginBottom: '1rem' }}>🤝 Join Our Community</h3>
        <p style={{ lineHeight: '1.8' }}>
          Whether you're a new cat owner or a seasoned cat lover, Cat World is here for you. 
          Explore our pages, learn about different breeds, discover care tips, and enjoy our adorable cat gallery. 
          Together, we can create a better world for all cats!
        </p>
      </motion.div>
      
      <motion.div 
        className="card"
        style={{ marginTop: '2rem' }}
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.6 }}
      >
        <h3 style={{ color: '#667eea', marginBottom: '1rem' }}>📬 Contact Us</h3>
        <p style={{ color: '#666', lineHeight: '1.8' }}>
          Have questions or want to share your cat's story? We'd love to hear from you! 
          Email us at <strong>hello@catworld.com</strong>
        </p>
      </motion.div>
    </motion.div>
  )
}

export default About
