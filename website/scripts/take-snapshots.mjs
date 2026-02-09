import fs from 'node:fs/promises'
import path from 'node:path'

const baseUrl = process.env.BASE_URL || 'http://127.0.0.1:3023'
const outDir = process.env.OUT_DIR || '/tmp/teacher-cv-snapshots'

const routes = [
  { name: 'home', hash: '#/' },
  { name: 'profile', hash: '#/profile' },
  { name: 'experience', hash: '#/experience' },
  { name: 'education', hash: '#/education' },
  { name: 'portfolio', hash: '#/portfolio' },
  { name: 'philosophy', hash: '#/philosophy' },
  { name: 'contact', hash: '#/contact' },
  { name: 'resume', hash: '#/resume' },
]

const viewports = [
  { label: 'desktop', width: 1280, height: 800, deviceScaleFactor: 1 },
  { label: 'mobile', width: 390, height: 844, deviceScaleFactor: 2 },
]

function slug(s) {
  return s.replace(/[^a-z0-9]+/gi, '-').replace(/^-+|-+$/g, '').toLowerCase()
}

async function warmLazyContent(page) {
  // Ensure below-the-fold lazy content is loaded so full-page screenshots
  // reflect what a real user sees when scrolling.
  await page.evaluate(async () => {
    const sleep = (ms) => new Promise(r => setTimeout(r, ms))
    const step = Math.max(240, Math.floor(window.innerHeight * 0.75))

    for (let y = 0; y < document.body.scrollHeight + step; y += step) {
      window.scrollTo(0, y)
      await sleep(40)
    }

    await sleep(80)
    window.scrollTo(0, 0)
  })
  await page.waitForTimeout(120)
}

async function applyFullPageSnapshotStyles(page) {
  // Playwright stitches multiple viewports for fullPage screenshots; sticky headers
  // get duplicated across the image. Make the header non-sticky for snapshots.
  await page.addStyleTag({
    content: `
      html { scroll-behavior: auto !important; }
      .site-header { position: absolute !important; }
    `,
  })
}

async function main() {
  // Playwright is provided via `npx -p playwright`.
  const { chromium } = await import('playwright')
  await fs.mkdir(outDir, { recursive: true })

  const browser = await chromium.launch()
  try {
    for (const vp of viewports) {
      const context = await browser.newContext({
        viewport: { width: vp.width, height: vp.height },
        deviceScaleFactor: vp.deviceScaleFactor,
      })
      const page = await context.newPage()

      // Prefer stable screenshots.
      await page.emulateMedia({ reducedMotion: 'reduce' })

      for (const r of routes) {
        const url = `${baseUrl}/${r.hash}`
        await page.goto(url, { waitUntil: 'networkidle' })
        await page.waitForTimeout(150)
        await applyFullPageSnapshotStyles(page)
        await warmLazyContent(page)

        const file = path.join(outDir, `${vp.label}__${slug(r.name)}.png`)
        await page.screenshot({ path: file, fullPage: true })
      }

      if (vp.label === 'mobile') {
        // Snapshot the mobile menu open state.
        await page.goto(`${baseUrl}/#/`, { waitUntil: 'networkidle' })
        await page.waitForTimeout(150)
        await warmLazyContent(page)
        await page.click('.mobile-menu-btn')
        await page.waitForTimeout(150)
        await page.screenshot({
          path: path.join(outDir, 'mobile__menu-open.png'),
          fullPage: true,
        })
      }

      await context.close()
    }
  } finally {
    await browser.close()
  }

  // eslint-disable-next-line no-console
  console.log(`Saved snapshots to: ${outDir}`)
}

main().catch(err => {
  // eslint-disable-next-line no-console
  console.error(err)
  process.exit(1)
})
