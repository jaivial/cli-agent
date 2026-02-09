import { useEffect, useRef } from 'preact/hooks'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'

// Register ScrollTrigger
gsap.registerPlugin(ScrollTrigger)

function prefersReducedMotion() {
  return typeof window !== 'undefined' &&
    window.matchMedia &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

// Custom easing functions
export const easings = {
  smooth: 'power3.out',
  bouncy: 'back.out(1.7)',
  elastic: 'elastic.out(1, 0.5)',
  expo: 'expo.out',
  circ: 'circ.out',
  slow: 'power2.inOut',
}

// Hook for fade up animation on scroll
export function useFadeUp(delay = 0, duration = 0.8) {
  const ref = useRef(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (prefersReducedMotion()) {
      gsap.set(element, { opacity: 1, y: 0 })
      return
    }

    gsap.fromTo(
      element,
      { opacity: 0, y: 60 },
      {
        opacity: 1,
        y: 0,
        duration,
        delay,
        ease: easings.smooth,
        scrollTrigger: {
          trigger: element,
          start: 'top 85%',
          toggleActions: 'play none none none',
        },
      }
    )

    return () => {
      ScrollTrigger.getAll().forEach(st => {
        if (st.trigger === element) st.kill()
      })
    }
  }, [delay, duration])

  return ref
}

// Hook for staggered children animation
export function useStaggerChildren(staggerDelay = 0.1, delay = 0) {
  const ref = useRef(null)

  useEffect(() => {
    const container = ref.current
    if (!container) return

    const children = container.children
    if (!children.length) return
    if (prefersReducedMotion()) {
      gsap.set(children, { opacity: 1, y: 0, scale: 1, clearProps: 'transform' })
      return
    }

    gsap.fromTo(
      children,
      { opacity: 0, y: 40, scale: 0.95 },
      {
        opacity: 1,
        y: 0,
        scale: 1,
        duration: 0.6,
        stagger: staggerDelay,
        delay,
        ease: easings.smooth,
        scrollTrigger: {
          trigger: container,
          start: 'top 80%',
          toggleActions: 'play none none none',
        },
      }
    )

    return () => {
      ScrollTrigger.getAll().forEach(st => {
        if (st.trigger === container) st.kill()
      })
    }
  }, [staggerDelay, delay])

  return ref
}

// Hook for scale fade animation
export function useScaleFade(delay = 0) {
  const ref = useRef(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (prefersReducedMotion()) {
      gsap.set(element, { opacity: 1, scale: 1 })
      return
    }

    gsap.fromTo(
      element,
      { opacity: 0, scale: 0.8 },
      {
        opacity: 1,
        scale: 1,
        duration: 0.8,
        delay,
        ease: easings.bouncy,
        scrollTrigger: {
          trigger: element,
          start: 'top 85%',
          toggleActions: 'play none none none',
        },
      }
    )

    return () => {
      ScrollTrigger.getAll().forEach(st => {
        if (st.trigger === element) st.kill()
      })
    }
  }, [delay])

  return ref
}

// Hook for slide in from side
export function useSlideIn(direction = 'left', delay = 0) {
  const ref = useRef(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (prefersReducedMotion()) {
      gsap.set(element, { opacity: 1, x: 0, y: 0, clearProps: 'transform' })
      return
    }

    const xOffset = direction === 'left' ? -100 : direction === 'right' ? 100 : 0
    const yOffset = direction === 'top' ? -100 : direction === 'bottom' ? 100 : 0

    gsap.fromTo(
      element,
      { opacity: 0, x: xOffset, y: yOffset },
      {
        opacity: 1,
        x: 0,
        y: 0,
        duration: 0.8,
        delay,
        ease: easings.expo,
        scrollTrigger: {
          trigger: element,
          start: 'top 85%',
          toggleActions: 'play none none none',
        },
      }
    )

    return () => {
      ScrollTrigger.getAll().forEach(st => {
        if (st.trigger === element) st.kill()
      })
    }
  }, [direction, delay])

  return ref
}

// Hook for hero entrance animation
export function useHeroEntrance() {
  const containerRef = useRef(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const targets = container.querySelectorAll('.hero-animate')
    if (prefersReducedMotion()) {
      gsap.set(targets, { opacity: 1, y: 0, scale: 1, clearProps: 'transform' })
      return
    }

    const tl = gsap.timeline({ defaults: { ease: easings.expo } })

    // Animate hero elements
    tl.fromTo(
      targets,
      { opacity: 0, y: 50, scale: 0.95 },
      { opacity: 1, y: 0, scale: 1, duration: 1, stagger: 0.15 }
    )

    return () => {
      tl.kill()
    }
  }, [])

  return containerRef
}

// Hook for floating animation
export function useFloating(duration = 3, yDistance = 15) {
  const ref = useRef(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (prefersReducedMotion()) {
      gsap.set(element, { y: 0, clearProps: 'transform' })
      return
    }

    const tl = gsap.timeline({ repeat: -1, yoyo: true })
    tl.to(element, {
      y: -yDistance,
      duration: duration / 2,
      ease: 'sine.inOut',
    }).to(element, {
      y: yDistance,
      duration: duration / 2,
      ease: 'sine.inOut',
    })

    return () => {
      tl.kill()
    }
  }, [duration, yDistance])

  return ref
}

// Hook for magnetic button effect
export function useMagneticButton() {
  const ref = useRef(null)

  useEffect(() => {
    const button = ref.current
    if (!button) return
    if (prefersReducedMotion()) return

    const handleMouseMove = (e) => {
      const rect = button.getBoundingClientRect()
      const x = e.clientX - rect.left - rect.width / 2
      const y = e.clientY - rect.top - rect.height / 2

      gsap.to(button, {
        x: x * 0.3,
        y: y * 0.3,
        duration: 0.3,
        ease: 'power2.out',
      })
    }

    const handleMouseLeave = () => {
      gsap.to(button, {
        x: 0,
        y: 0,
        duration: 0.5,
        ease: easings.elastic,
      })
    }

    button.addEventListener('mousemove', handleMouseMove)
    button.addEventListener('mouseleave', handleMouseLeave)

    return () => {
      button.removeEventListener('mousemove', handleMouseMove)
      button.removeEventListener('mouseleave', handleMouseLeave)
    }
  }, [])

  return ref
}

// Hook for text reveal animation
export function useTextReveal(delay = 0) {
  const ref = useRef(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (prefersReducedMotion()) return

    const chars = element.textContent.split('')
    element.innerHTML = chars.map(char => 
      char === ' ' ? ' ' : `<span class="char" style="display: inline-block; opacity: 0; transform: translateY(20px);">${char}</span>`
    ).join('')

    gsap.to(element.querySelectorAll('.char'), {
      opacity: 1,
      y: 0,
      duration: 0.5,
      stagger: 0.03,
      delay,
      ease: easings.smooth,
      scrollTrigger: {
        trigger: element,
        start: 'top 85%',
        toggleActions: 'play none none none',
      },
    })

    return () => {
      ScrollTrigger.getAll().forEach(st => {
        if (st.trigger === element) st.kill()
      })
    }
  }, [delay])

  return ref
}

// Hook for pulse/glow animation
export function usePulseGlow(color = 'var(--accent-2)') {
  const ref = useRef(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (prefersReducedMotion()) return

    const tl = gsap.timeline({ repeat: -1, yoyo: true })
    tl.to(element, {
      boxShadow: `0 0 30px ${color}40, 0 0 60px ${color}20`,
      duration: 1.5,
      ease: 'sine.inOut',
    }).to(element, {
      boxShadow: `0 0 10px ${color}20, 0 0 20px ${color}10`,
      duration: 1.5,
      ease: 'sine.inOut',
    })

    return () => {
      tl.kill()
    }
  }, [color])

  return ref
}

// Hook for parallax effect
export function useParallax(speed = 0.5) {
  const ref = useRef(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (prefersReducedMotion()) return

    const handleScroll = () => {
      const scrolled = window.scrollY
      const rect = element.getBoundingClientRect()
      const elementTop = rect.top + scrolled
      const relativeScroll = scrolled - elementTop + window.innerHeight

      gsap.to(element, {
        y: relativeScroll * speed * 0.1,
        duration: 0.1,
        ease: 'none',
      })
    }

    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => window.removeEventListener('scroll', handleScroll)
  }, [speed])

  return ref
}

// Hook for card hover 3D tilt effect
export function useTiltCard() {
  const ref = useRef(null)

  useEffect(() => {
    const card = ref.current
    if (!card) return
    if (prefersReducedMotion()) return

    const handleMouseMove = (e) => {
      const rect = card.getBoundingClientRect()
      const x = e.clientX - rect.left
      const y = e.clientY - rect.top
      const centerX = rect.width / 2
      const centerY = rect.height / 2
      const rotateX = (y - centerY) / 10
      const rotateY = (centerX - x) / 10

      gsap.to(card, {
        rotateX: rotateX,
        rotateY: rotateY,
        duration: 0.3,
        ease: 'power2.out',
        transformPerspective: 1000,
      })
    }

    const handleMouseLeave = () => {
      gsap.to(card, {
        rotateX: 0,
        rotateY: 0,
        duration: 0.5,
        ease: easings.elastic,
      })
    }

    card.addEventListener('mousemove', handleMouseMove)
    card.addEventListener('mouseleave', handleMouseLeave)

    return () => {
      card.removeEventListener('mousemove', handleMouseMove)
      card.removeEventListener('mouseleave', handleMouseLeave)
    }
  }, [])

  return ref
}

// Clean up all ScrollTriggers on unmount
export function cleanupScrollTriggers() {
  ScrollTrigger.getAll().forEach(st => st.kill())
}
