import { useEffect, useState } from 'preact/hooks'
import { Copy, Check } from './Icons'

export default function CopyButton({ value, label = 'Copy' }) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const t = window.setTimeout(() => setCopied(false), 1400)
    return () => window.clearTimeout(t)
  }, [copied])

  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
    } catch {
      // Fallback: select + copy
      const el = document.createElement('textarea')
      el.value = value
      el.setAttribute('readonly', '')
      el.style.position = 'fixed'
      el.style.top = '-1000px'
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
      setCopied(true)
    }
  }

  return (
    <button
      className="btn btn-ghost btn-icon"
      type="button"
      onClick={onCopy}
      aria-label={copied ? 'Copied to clipboard' : label}
    >
      {copied ? <Check size={18} /> : <Copy size={18} />}
      <span>{copied ? 'Copied' : label}</span>
      <span className="sr-only" role="status" aria-live="polite">
        {copied ? 'Copied to clipboard' : ''}
      </span>
    </button>
  )
}
