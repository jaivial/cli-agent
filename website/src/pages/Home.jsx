import React from 'react'
import { motion } from 'framer-motion'
import { Link } from 'react-router-dom'

const Home = () => {
  return (
    <motion.div 
      className="page-container"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      transition={{ duration: 0.5 }}
    >
      <motion.h1 
        className="page-title"
        initial={{ scale: 0.5 }}
        animate={{ scale: 1 }}
        transition={{ type: 'spring', stiffness: 200 }}
      >
        Welcome to Cat World! 🐱
      </motion.h1>
      
      <motion.img 
        src="https://st4.depositphotos.com/13193636/23666/i/450/depositphotos_236669390-stock-photo-beautiful-cat.jpg"
        alt="Beautiful cat"
        className="page-image"
        initial={{ opacity: 0, scale: 0.8 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ delay: 0.2, duration: 0.6 }}
      />
      
      <motion.p 
        style={{ fontSize: '1.2rem', lineHeight: '1.8', marginBottom: '2rem' }}
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.4 }}
      >
        Discover the wonderful world of cats! From different breeds to care tips, 
        adorable galleries, and interesting facts. Cats have been our companions for thousands of years, 
        bringing joy, comfort, and endless entertainment to our lives.
      </motion.p>
      
      <div className="grid-3">
        {[
          { title: 'Explore Breeds', desc: 'Learn about different cat breeds', path: '/breeds', emoji: '🐈' },
          { title: 'Care Tips', desc: 'How to take care of your cat', path: '/care', emoji: '💕' },
          { title: 'Gallery', desc: 'Adorable cat pictures', path: '/gallery', emoji: '📸' },
          { title: 'About Us', desc: 'Learn more about Cat World', path: '/about', emoji: 'ℹ️' },
          { title: 'Cat Facts', desc: 'Interesting cat trivia', path: '/breeds', emoji: '🧠' },
          { title: 'Adoption', desc: 'Find your perfect companion', path: '/about', emoji: '🏠' },
        ].map((item, index) => (
          <motion.div 
            key={item.title}
            className="card"
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5 + (index * 0.1) }}
            whileHover={{ scale: 1.05 }}
          >
            <Link to={item.path} style={{ textDecoration: 'none', color: 'inherit' }}>
              <h3 style={{ fontSize: '1.5rem', marginBottom: '0.5rem', color: '#667eea' }}>
                {item.emoji} {item.title}
              </h3>
              <p style={{ color: '#666' }}>{item.desc}</p>
            </Link>
          </motion.div>
        ))}
      </div>
    </motion.div>
  )
}

export default Home
