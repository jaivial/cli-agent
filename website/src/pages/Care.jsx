import React from 'react'
import { motion } from 'framer-motion'

const careTips = [
  {
    title: 'Nutrition',
    image: 'https://st4.depositphotos.com/1010612/28539/i/450/depositphotos_285390256-stock-photo-cat-eating-food-from-bowl.jpg',
    tips: [
      'Provide high-quality cat food appropriate for their age',
      'Fresh water should always be available',
      'Avoid feeding them toxic foods like chocolate, onions, or garlic',
      'Consider wet food for hydration',
      'Follow feeding guidelines and monitor weight'
    ]
  },
  {
    title: 'Grooming',
    image: 'https://st4.depositphotos.com/1688730/4423/i/450/depositphotos_44235213-stock-photo-cat-grooming.jpg',
    tips: [
      'Brush long-haired cats daily to prevent mats',
      'Trim nails every 2-3 weeks',
      'Check ears weekly for dirt or signs of infection',
      'Bathe only when necessary (most cats self-clean)',
      'Start grooming routines early for kittens'
    ]
  },
  {
    title: 'Health',
    image: 'https://st4.depositphotos.com/13193636/31869/i/450/depositphotos_318691422-stock-photo-veterinarian-examining-cat.jpg',
    tips: [
      'Schedule regular veterinary checkups',
      'Keep vaccinations up to date',
      'Spay or neuter your cat',
      'Watch for changes in behavior or appetite',
      'Provide flea and tick prevention'
    ]
  },
  {
    title: 'Environment',
    image: 'https://st4.depositphotos.com/27350638/38970/i/450/depositphotos_389701758-stock-photo-happy-cat-sitting-by-window.jpg',
    tips: [
      'Create a safe, stimulating environment',
      'Provide scratching posts and toys',
      'Ensure they have quiet places to rest',
      'Keep toxic plants out of reach',
      'Consider vertical space with cat trees'
    ]
  }
]

const Care = () => {
  return (
    <motion.div 
      className="page-container"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      transition={{ duration: 0.5 }}
    >
      <h1 className="page-title">Cat Care Guide 💕</h1>
      <p style={{ textAlign: 'center', marginBottom: '2rem', fontSize: '1.1rem' }}>
        Everything you need to know to keep your feline friend happy and healthy
      </p>
      
      <div className="grid-2">
        {careTips.map((category, index) => (
          <motion.div 
            key={category.title}
            className="card"
            initial={{ opacity: 0, x: -30 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: index * 0.15, duration: 0.5 }}
          >
            <img 
              src={category.image} 
              alt={category.title}
              style={{ width: '100%', height: '200px', objectFit: 'cover', borderRadius: '10px' }}
            />
            <h2 style={{ color: '#667eea', margin: '1rem 0 0.5rem' }}>{category.title}</h2>
            <ul style={{ listStyle: 'none', lineHeight: '1.8', color: '#666' }}>
              {category.tips.map((tip, tipIndex) => (
                <motion.li 
                  key={tipIndex}
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: 0.5 + (index * 0.15) + (tipIndex * 0.05) }}
                >
                  ✓ {tip}
                </motion.li>
              ))}
            </ul>
          </motion.div>
        ))}
      </div>
      
      <motion.div 
        className="card"
        style={{ marginTop: '2rem', background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', color: 'white' }}
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ delay: 1.2 }}
      >
        <h3 style={{ marginBottom: '1rem' }}>💡 Pro Tips</h3>
        <ul style={{ listStyle: 'none', lineHeight: '2' }}>
          <li>• Spend quality time playing with your cat daily</li>
          <li>• Learn to read your cat's body language</li>
          <li>• Keep litter boxes clean - scoop daily!</li>
          <li>• Microchip your cat for identification</li>
          <li>• Create an emergency kit for your pet</li>
        </ul>
      </motion.div>
    </motion.div>
  )
}

export default Care
