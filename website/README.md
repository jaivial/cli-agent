# Teacher CV Website

CV website template for an **early years / preschool teacher (ages 3–4)**.

## Routes

- `/` Home (snapshot)
- `/profile` Profile
- `/experience` Experience
- `/education` Education & certifications
- `/portfolio` Portfolio (classroom activities)
- `/philosophy` Teaching philosophy
- `/contact` Contact
- `/resume` Print-friendly resume (use “Print / Save PDF”)

## Edit Your Details

- Update content in `website/src/content/cv.js`.

## Tech

- Vite + Preact (React-compatible JSX + hooks)
- `preact-router` with hash history (`#/profile`, etc.) for static hosting
- GSAP hooks included for simple entrance motion (respects `prefers-reduced-motion`)

## Development

```bash
cd website
npm run dev
```

## Build

```bash
cd website
npm run build
npm run preview
```

