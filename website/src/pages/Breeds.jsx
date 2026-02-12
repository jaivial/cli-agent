import React from 'react'
import { motion } from 'framer-motion'

const breeds = [
  {
    name: 'Persian',
    image: 'https://st4.depositphotos.com/15933568/24616/i/450/depositphotos_246166590-stock-photo-white-persian-cat.jpg',
    description: 'Known for their long, luxurious coats and gentle, calm personalities. They make wonderful indoor companions.'
  },
  {
    name: 'Siamese',
    image: 'https://st4.depositphotos.com/1010612/2651/i/450/depositphotos_26514799-stock-photo-siamese-cat.jpg',
    description: 'Famous for their striking blue eyes and vocal nature. They are highly social and love attention.'
  },
  {
    name: 'Maine Coon',
    image: 'https://st4.depositphotos.com/27350638/38956/i/450/depositphotos_389569416-stock-photo-maine-coon-cat.jpg',
    description: 'One of the largest domestic cat breeds. They are friendly, gentle giants with tufted ears and bushy tails.'
  },
  {
    name: 'British Shorthair',
    image: 'https://st4.depositphotos.com/13356592/23684/i/450/depositphotos_236844980-stock-photo-british-shorthair-cat.jpg',
    description: 'Easygoing and affectionate with a round face and dense coat. They are the perfect family pets.'
  },
  {
    name: 'Bengal',
    image: 'https://st4.depositphotos.com/17555534/201405/i/450/depositphotos_201405098-stock-photo-bengal-cat.jpg',
    description: 'Known for their leopard-like spotted coat. They are energetic, intelligent, and love to play.'
  },
  {
    name: 'Ragdoll',
    image: 'https://st4.depositphotos.com/1010612/3857/i/450/depositphotos_38578291-stock-photo-ragdoll-cat.jpg',
    description: 'Large, affectionate cats that go limp when picked up. They have beautiful blue eyes and semi-longhair coats.'
  }
]

const Breeds = () => {
  return (
    <motion.div 
      className="page-container"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      transition={{ duration: 0.5 }}
    >
      <h1 className="page-title">Cat Breeds 🐱</h1>
      <p style={{ textAlign: 'center', marginBottom: '2rem', fontSize: '1.1rem' }}>
        Explore the diverse and beautiful world of cat breeds
      </p>
      
      <div className="grid-2">
        {breeds.map((breed, index) => (
          <motion.div 
            key={breed.name}
            className="card"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: index * 0.1, duration: 0.4 }}
            whileHover={{ scale: 1.03 }}
          >
            <img 
              src={breed.image} 
              alt={breed.name}
              style={{ width: '100%', height: '250px', objectFit: 'cover', borderRadius: '10px' }}
            />
            <h2 style={{ color: '#667eea', margin: '1rem 0 0.5rem' }}>{breed.name}</h2>
            <p style={{ color: '#666', lineHeight: '1.6' }}>{breed.description}</p>
          </motion.div>
        ))}
      </div>
      
      <motion.div 
        className="card"
        style={{ marginTop: '2rem', background: '#667eea', color: 'white' }}
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.6 }}
      >
        <h3 style={{ marginBottom: '1rem' }}>🧠 Fun Cat Facts</h3>
        <ul style={{ listStyle: 'none', lineHeight: '2' }}>
          <li>• Cats spend 70% of their lives sleeping</li>
          <li>• A group of cats is called a "clowder"</li>
          <li>• Cats can rotate their ears 180 degrees</li>
          <li>• The oldest known cat lived to be 38 years old</li>
          <li>• Cats have over 20 different vocalizations</li>
        </ul>
      </motion.div>
    </motion.div>
  )
}

export default Breeds
