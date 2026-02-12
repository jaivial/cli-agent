import React from 'react'
import { BrowserRouter, Routes, Route, Link } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import './App.css'

// Page Components
import Home from './pages/Home'
import Breeds from './pages/Breeds'
import Care from './pages/Care'
import Gallery from './pages/Gallery'
import About from './pages/About'
import { CarnivalLayout } from './pages/carnival/components/CarnivalLayout'
import CarnivalHome from './pages/carnival'
import CarnivalRoutes from './pages/carnival/CarnivalRoutes'
import CarnivalSchedule from './pages/carnival/CarnivalSchedule'
import CarnivalTips from './pages/carnival/CarnivalTips'
import SambodromoPage from './pages/carnival/SambodromoPage'
import CentroLapaPage from './pages/carnival/CentroLapaPage'
import OrlaPage from './pages/carnival/OrlaPage'

function App() {
  return (
    <BrowserRouter>
      <div className="app">
        <nav className="navbar">
          <motion.div 
            className="nav-container"
            initial={{ y: -100 }}
            animate={{ y: 0 }}
            transition={{ type: 'spring', stiffness: 100 }}
          >
            <motion.h1 
              className="logo"
              whileHover={{ scale: 1.1 }}
            >
              🐱 Cat World
            </motion.h1>
            <ul className="nav-links">
              {[{path: '/', label: 'Home'}, {path: '/breeds', label: 'Breeds'}, {path: '/care', label: 'Care'}, {path: '/gallery', label: 'Gallery'}, {path: '/about', label: 'About'}, {path: '/carnival', label: 'Carnival'}].map((item) => (
                <motion.li
                  key={item.path}
                  whileHover={{ scale: 1.1 }}
                  whileTap={{ scale: 0.95 }}
                >
                  <Link to={item.path}>{item.label}</Link>
                </motion.li>
              ))}
            </ul>
          </motion.div>
        </nav>
        <main className="main-content">
          <AnimatePresence mode="wait">
            <Routes>
              <Route path="/" element={<Home />} />
              <Route path="/breeds" element={<Breeds />} />
              <Route path="/care" element={<Care />} />
              <Route path="/gallery" element={<Gallery />} />
              <Route path="/about" element={<About />} />
              <Route path="/carnival" element={<CarnivalHome />} />
              <Route path="/carnival/sambodromo" element={<SambodromoPage />} />
              <Route path="/carnival/centro-lapa" element={<CentroLapaPage />} />
              <Route path="/carnival/orla" element={<OrlaPage />} />
              <Route path="/carnival/routes" element={<CarnivalLayout><CarnivalRoutes /></CarnivalLayout>} />
              <Route path="/carnival/schedule" element={<CarnivalLayout><CarnivalSchedule /></CarnivalLayout>} />
              <Route path="/carnival/tips" element={<CarnivalLayout><CarnivalTips /></CarnivalLayout>} />
            </Routes>
          </AnimatePresence>
        </main>
        <footer className="footer">
          <p>© 2024 Cat World - Images from DepositPhotos</p>
        </footer>
      </div>
    </BrowserRouter>
  )
}

export default App
