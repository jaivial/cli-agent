package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func looksLikeReactWebsiteScaffoldRequest(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return false
	}
	if !(strings.Contains(s, "react") || strings.Contains(s, "vite") || strings.Contains(s, "next.js") || strings.Contains(s, "nextjs")) {
		return false
	}
	if !(strings.Contains(s, "website") ||
		strings.Contains(s, "web site") ||
		strings.Contains(s, "webstie") || // common typo
		strings.Contains(s, "webiste") || // common typo
		strings.Contains(s, "landing page") ||
		strings.Contains(s, "site") ||
		strings.Contains(s, "app") ||
		strings.Contains(s, "project")) {
		// Avoid triggering on incidental React mentions.
		return false
	}
	verbs := []string{"create", "build", "make", "generate", "scaffold", "start"}
	for _, v := range verbs {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

func slugifyFolderName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	// Turn separators into dashes.
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	// Remove disallowed characters.
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if r == '-' {
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
			continue
		}
	}
	out := strings.Trim(b.String(), "-")
	out = strings.ReplaceAll(out, "--", "-")
	return out
}

func titleCaseWords(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	for i := range parts {
		p := parts[i]
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

func inferReactScaffoldDirAndTitle(input string) (dir string, title string) {
	s := strings.TrimSpace(input)
	low := strings.ToLower(s)

	// Prefer explicit folder name: `folder named foo` or `folder called foo`.
	reNamed := regexp.MustCompile("(?i)(?:folder|directory|project|app)\\s+(?:named|called)\\s*[\"'`]?([a-z0-9][a-z0-9_-]{0,63})[\"'`]?")
	if m := reNamed.FindStringSubmatch(s); len(m) == 2 {
		dir = slugifyFolderName(m[1])
	}
	// Also accept quoted folder names: `folder \"foo\"`, `directory 'foo'`, etc.
	if dir == "" {
		reQuoted := regexp.MustCompile("(?i)(?:folder|directory|project|app)\\s+[\"'`]([a-z0-9][a-z0-9_-]{0,63})[\"'`]")
		if m := reQuoted.FindStringSubmatch(s); len(m) == 2 {
			dir = slugifyFolderName(m[1])
		}
	}

	// Common phrase: "in a new folder for my pet store" => pet-store.
	if dir == "" && strings.Contains(low, "pet store") {
		dir = "pet-store"
		title = "Pet Store"
	}

	// "for my X" => derive both title and slug.
	if dir == "" {
		reFor := regexp.MustCompile(`(?i)\bfor\s+my\s+([a-z0-9][a-z0-9 \t_-]{0,64})`)
		if m := reFor.FindStringSubmatch(s); len(m) == 2 {
			cand := strings.TrimSpace(m[1])
			cand = strings.Trim(cand, ".!?")
			if cand != "" {
				// Keep title from words, but avoid trailing generic words.
				title = titleCaseWords(cand)
				dir = slugifyFolderName(cand)
			}
		}
	}

	if dir == "" {
		dir = "react-app"
	}
	if title == "" {
		// If we inferred a dir from an explicit token, make a reasonable title.
		title = titleCaseWords(strings.ReplaceAll(dir, "-", " "))
		if title == "" {
			title = "React App"
		}
	}
	return dir, title
}

func writeFileMkdirAll(path string, content []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, perm)
}

func ensureDirEmptyOrMissing(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", path)
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(ents) != 0 {
		return fmt.Errorf("directory %s already exists and is not empty", path)
	}
	return nil
}

func scaffoldViteReactApp(dir string, title string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("missing directory name")
	}
	if err := ensureDirEmptyOrMissing(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	pkg := map[string]any{
		"name":    slugifyFolderName(filepath.Base(dir)),
		"private": true,
		"version": "0.0.0",
		"type":    "module",
		"scripts": map[string]string{
			"dev":     "vite",
			"build":   "vite build",
			"preview": "vite preview",
		},
		"dependencies": map[string]string{
			"react":     "^18.2.0",
			"react-dom": "^18.2.0",
		},
		"devDependencies": map[string]string{
			"@vitejs/plugin-react": "^4.2.1",
			"vite":                "^5.2.0",
		},
	}
	pkgJSON, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return nil, err
	}

	indexHTML := fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>%s</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.jsx"></script>
  </body>
</html>
`, title)

	viteConfig := `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
});
`

	readme := fmt.Sprintf("# %s\n\nLocal dev:\n\n```bash\ncd %s\nnpm install\nnpm run dev\n```\n\nBuild:\n\n```bash\nnpm run build\nnpm run preview\n```\n", title, dir)

	gitignore := `node_modules
dist
.DS_Store
.env
.env.*
`

	mainJSX := `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App.jsx";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
`

	appJSX := fmt.Sprintf(`export default function App() {
  return (
    <div className="page">
      <header className="top">
        <div className="brand">
          <span className="mark" aria-hidden="true">P</span>
          <div className="brandText">
            <div className="name">%s</div>
            <div className="tag">Thoughtful gear, treats, and care essentials</div>
          </div>
        </div>
        <nav className="nav" aria-label="Primary">
          <a href="#shop">Shop</a>
          <a href="#bundles">Bundles</a>
          <a href="#about">About</a>
        </nav>
      </header>

      <main className="main">
        <section className="hero" aria-labelledby="hero-title">
          <div className="heroCopy">
            <h1 id="hero-title">Everything your best friend needs, beautifully curated.</h1>
            <p>
              Discover durable toys, clean ingredients, and cozy comfort picks. Fast shipping and
              easy returns.
            </p>
            <div className="ctaRow">
              <a className="btn primary" href="#shop">Shop new arrivals</a>
              <a className="btn ghost" href="#bundles">Build a bundle</a>
            </div>
            <div className="proof">
              <div className="pill">Free shipping $49+</div>
              <div className="pill">Vet-approved picks</div>
              <div className="pill">30-day returns</div>
            </div>
          </div>
          <div className="heroCard" role="img" aria-label="Pet store highlight card">
            <div className="cardTop">
              <div className="kicker">Featured</div>
              <div className="cardTitle">Calm + Cozy Starter Kit</div>
            </div>
            <ul className="cardList">
              <li>Soft-touch bed cover</li>
              <li>Freeze-dried salmon bites</li>
              <li>Quiet chew toy</li>
            </ul>
            <div className="cardBottom">
              <div className="price">
                <div className="priceNow">$39</div>
                <div className="priceWas">was $52</div>
              </div>
              <button className="btn small" type="button">Add to cart</button>
            </div>
          </div>
        </section>

        <section id="shop" className="grid" aria-label="Shop categories">
          <a className="tile" href="#shop">
            <div className="tileTitle">Treats</div>
            <div className="tileBody">Clean ingredients, big crunch.</div>
          </a>
          <a className="tile" href="#shop">
            <div className="tileTitle">Toys</div>
            <div className="tileBody">Tough, safe, and actually fun.</div>
          </a>
          <a className="tile" href="#shop">
            <div className="tileTitle">Groom</div>
            <div className="tileBody">Shampoo bars, brushes, and balm.</div>
          </a>
          <a className="tile" href="#shop">
            <div className="tileTitle">Travel</div>
            <div className="tileBody">Leashes, bowls, and carriers.</div>
          </a>
        </section>

        <section id="bundles" className="banner" aria-labelledby="bundle-title">
          <div>
            <h2 id="bundle-title">Bundle and save</h2>
            <p>Pick 3+ items and get automatic tiered discounts at checkout.</p>
          </div>
          <a className="btn primary" href="#shop">Start bundling</a>
        </section>

        <section id="about" className="about" aria-label="About">
          <h2>Made for real life</h2>
          <p>
            We test products with picky pets and practical humans. If we would not use it in our
            own home, it does not make the shelf.
          </p>
        </section>
      </main>

      <footer className="footer">
        <div className="footerInner">
          <div className="fine">© {new Date().getFullYear()} %s</div>
          <div className="fine">Built with React + Vite</div>
        </div>
      </footer>
    </div>
  );
}
`, title, title)

	css := `:root {
  --bg: #fbf8f2;
  --ink: #142018;
  --muted: rgba(20, 32, 24, 0.7);
  --card: rgba(255, 255, 255, 0.72);
  --stroke: rgba(20, 32, 24, 0.16);
  --shadow: 0 14px 40px rgba(20, 32, 24, 0.18);
  --accent: #1d6b55;
  --accent2: #e24c3a;
  --radius: 18px;
  --max: 1100px;
}

* { box-sizing: border-box; }

html, body {
  height: 100%;
  margin: 0;
  color: var(--ink);
  background:
    radial-gradient(1100px 600px at 20% -10%, rgba(29, 107, 85, 0.28), transparent 55%),
    radial-gradient(900px 520px at 92% 10%, rgba(226, 76, 58, 0.22), transparent 60%),
    radial-gradient(1000px 650px at 40% 110%, rgba(233, 201, 96, 0.25), transparent 60%),
    var(--bg);
  font-family: ui-serif, "Iowan Old Style", "Palatino Linotype", Palatino, Georgia, serif;
}

a { color: inherit; text-decoration: none; }
a:hover { text-decoration: underline; }

.page {
  min-height: 100%;
  display: flex;
  flex-direction: column;
}

.top {
  max-width: var(--max);
  width: calc(100% - 40px);
  margin: 22px auto 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 14px 16px;
  border: 1px solid var(--stroke);
  border-radius: calc(var(--radius) + 8px);
  background: rgba(255, 255, 255, 0.55);
  backdrop-filter: blur(10px);
}

.brand { display: flex; align-items: center; gap: 12px; }
.mark {
  width: 36px; height: 36px;
  display: grid; place-items: center;
  border-radius: 12px;
  background: linear-gradient(135deg, var(--accent), #2aa77f);
  color: white;
  font-weight: 700;
  letter-spacing: 0.02em;
  box-shadow: 0 12px 22px rgba(29, 107, 85, 0.22);
}
.brandText .name { font-weight: 700; letter-spacing: 0.01em; }
.brandText .tag { font-size: 13px; color: var(--muted); margin-top: 2px; }

.nav { display: flex; gap: 14px; font-size: 14px; }
.nav a { padding: 8px 10px; border-radius: 12px; }
.nav a:hover { background: rgba(20, 32, 24, 0.06); text-decoration: none; }

.main {
  max-width: var(--max);
  width: calc(100% - 40px);
  margin: 18px auto 0;
  padding: 14px 0 46px;
  flex: 1;
}

.hero {
  display: grid;
  grid-template-columns: 1.15fr 0.85fr;
  gap: 18px;
  align-items: stretch;
  margin-top: 14px;
}

.heroCopy {
  padding: 22px 18px;
  border-radius: calc(var(--radius) + 6px);
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.5);
  backdrop-filter: blur(10px);
}
.heroCopy h1 {
  margin: 0;
  font-size: clamp(28px, 3.5vw, 44px);
  line-height: 1.05;
  letter-spacing: -0.02em;
}
.heroCopy p { margin: 14px 0 0; color: var(--muted); font-size: 16px; line-height: 1.55; max-width: 58ch; }

.ctaRow { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 16px; }
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: 14px;
  padding: 10px 14px;
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.64);
  cursor: pointer;
  font-weight: 650;
}
.btn:hover { transform: translateY(-1px); box-shadow: 0 10px 24px rgba(20, 32, 24, 0.14); text-decoration: none; }
.btn.primary { background: linear-gradient(135deg, var(--accent), #2aa77f); color: white; border-color: rgba(0, 0, 0, 0.06); }
.btn.ghost { background: rgba(255, 255, 255, 0.2); }
.btn.small { padding: 9px 12px; border-radius: 13px; }

.proof { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
.pill {
  font-size: 12px;
  padding: 7px 10px;
  border: 1px solid var(--stroke);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.55);
}

.heroCard {
  border-radius: calc(var(--radius) + 8px);
  border: 1px solid var(--stroke);
  background:
    radial-gradient(120px 120px at 10% 20%, rgba(226, 76, 58, 0.22), transparent 60%),
    radial-gradient(160px 160px at 85% 15%, rgba(29, 107, 85, 0.22), transparent 60%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.66), rgba(255, 255, 255, 0.46));
  backdrop-filter: blur(10px);
  padding: 16px;
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}
.kicker { font-size: 12px; color: var(--muted); letter-spacing: 0.08em; text-transform: uppercase; }
.cardTitle { margin-top: 6px; font-weight: 800; font-size: 18px; letter-spacing: -0.01em; }
.cardList { margin: 14px 0 0; padding-left: 18px; color: rgba(20, 32, 24, 0.78); }
.cardBottom { display: flex; justify-content: space-between; align-items: center; margin-top: 14px; gap: 12px; }
.priceNow { font-weight: 850; font-size: 20px; }
.priceWas { font-size: 12px; color: var(--muted); }

.grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-top: 16px;
}
.tile {
  padding: 16px;
  border-radius: calc(var(--radius) + 6px);
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.48);
  backdrop-filter: blur(10px);
  min-height: 110px;
}
.tile:hover { box-shadow: 0 14px 30px rgba(20, 32, 24, 0.14); text-decoration: none; }
.tileTitle { font-weight: 850; letter-spacing: -0.01em; }
.tileBody { margin-top: 8px; color: var(--muted); font-size: 14px; line-height: 1.45; }

.banner {
  margin-top: 16px;
  padding: 18px 16px;
  border-radius: calc(var(--radius) + 8px);
  border: 1px solid var(--stroke);
  background: linear-gradient(135deg, rgba(29, 107, 85, 0.16), rgba(226, 76, 58, 0.10));
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}
.banner h2 { margin: 0; letter-spacing: -0.01em; }
.banner p { margin: 6px 0 0; color: var(--muted); }

.about {
  margin-top: 18px;
  padding: 18px 16px;
  border-radius: calc(var(--radius) + 8px);
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.44);
  backdrop-filter: blur(10px);
}
.about h2 { margin: 0; letter-spacing: -0.01em; }
.about p { margin: 10px 0 0; color: var(--muted); line-height: 1.6; max-width: 75ch; }

.footer {
  padding: 20px 0 34px;
}
.footerInner {
  max-width: var(--max);
  width: calc(100% - 40px);
  margin: 0 auto;
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--muted);
  font-size: 13px;
}

@media (max-width: 900px) {
  .hero { grid-template-columns: 1fr; }
  .grid { grid-template-columns: repeat(2, 1fr); }
  .footerInner { flex-direction: column; }
  .nav { display: none; }
}

@media (max-width: 520px) {
  .grid { grid-template-columns: 1fr; }
  .top { padding: 12px 12px; }
}
`

	files := map[string][]byte{
		filepath.Join(dir, "package.json"):      append(pkgJSON, '\n'),
		filepath.Join(dir, "vite.config.js"):    []byte(viteConfig),
		filepath.Join(dir, "index.html"):        []byte(indexHTML),
		filepath.Join(dir, "README.md"):         []byte(readme),
		filepath.Join(dir, ".gitignore"):        []byte(gitignore),
		filepath.Join(dir, "src", "main.jsx"):   []byte(mainJSX),
		filepath.Join(dir, "src", "App.jsx"):    []byte(appJSX),
		filepath.Join(dir, "src", "index.css"):  []byte(css),
	}

	created := make([]string, 0, len(files))
	for path, content := range files {
		if err := writeFileMkdirAll(path, content, 0o644); err != nil {
			return nil, err
		}
		created = append(created, path)
	}
	return created, nil
}

func tryLocalReactScaffold(input string) (string, bool, error) {
	if !looksLikeReactWebsiteScaffoldRequest(input) {
		return "", false, nil
	}
	dir, title := inferReactScaffoldDirAndTitle(input)
	created, err := scaffoldViteReactApp(dir, title)
	if err != nil {
		return "", true, err
	}

	// Return "Created <abs>:1" lines for a clickable UX.
	var lines []string
	for _, p := range created {
		abs := p
		if !filepath.IsAbs(abs) {
			if a, err := filepath.Abs(p); err == nil {
				abs = a
			}
		}
		lines = append(lines, fmt.Sprintf("Created %s:1", abs))
	}
	lines = append(lines, fmt.Sprintf("Next: `cd %s && npm install && npm run dev`", dir))
	return strings.Join(lines, "\n"), true, nil
}
