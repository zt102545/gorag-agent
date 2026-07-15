const allowedTags = new Set([
  'A',
  'B',
  'BLOCKQUOTE',
  'BR',
  'CODE',
  'DIV',
  'EM',
  'I',
  'IMG',
  'LI',
  'OL',
  'P',
  'PRE',
  'S',
  'SPAN',
  'STRONG',
  'U',
  'UL',
])

function isHttpUrl(value: string) {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

function looksLikeHtml(value: string) {
  return /<\/?[a-z][\s\S]*>/i.test(value)
}

const imageUrlPattern = /https?:\/\/[^\s<>"']+\.(?:avif|gif|jpe?g|png|svg|webp)(?:\?[^\s<>"']*)?(?:#[^\s<>"']*)?/gi

function enrichText(value: string, doc: Document) {
  const fragment = doc.createDocumentFragment()
  let cursor = 0
  imageUrlPattern.lastIndex = 0
  for (const match of value.matchAll(imageUrlPattern)) {
    const index = match.index ?? 0
    if (index > cursor) fragment.appendChild(doc.createTextNode(value.slice(cursor, index)))
    const src = match[0]
    const link = doc.createElement('a')
    link.setAttribute('href', src)
    link.setAttribute('target', '_blank')
    link.setAttribute('rel', 'noopener noreferrer')
    const image = doc.createElement('img')
    image.setAttribute('src', src)
    image.setAttribute('alt', '客服图片')
    image.setAttribute('loading', 'lazy')
    image.setAttribute('decoding', 'async')
    link.appendChild(image)
    fragment.appendChild(link)
    cursor = index + src.length
  }
  if (cursor < value.length) fragment.appendChild(doc.createTextNode(value.slice(cursor)))
  return fragment
}

function sanitizeNode(node: Node, doc: Document): Node | null {
  if (node.nodeType === Node.TEXT_NODE) {
    return enrichText(node.textContent || '', doc)
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return null

  const el = node as HTMLElement
  if (!allowedTags.has(el.tagName)) {
    const fragment = doc.createDocumentFragment()
    el.childNodes.forEach((child) => {
      const clean = sanitizeNode(child, doc)
      if (clean) fragment.appendChild(clean)
    })
    return fragment
  }

  const clean = doc.createElement(el.tagName.toLowerCase())
  if (el.tagName === 'A') {
    const href = el.getAttribute('href') || ''
    if (isHttpUrl(href)) {
      clean.setAttribute('href', href)
      clean.setAttribute('target', '_blank')
      clean.setAttribute('rel', 'noopener noreferrer')
    }
  }
  if (el.tagName === 'IMG') {
    const src = el.getAttribute('src') || ''
    if (!isHttpUrl(src)) return null
    clean.setAttribute('src', src)
    clean.setAttribute('alt', el.getAttribute('alt') || '')
    clean.setAttribute('loading', 'lazy')
  }

  el.childNodes.forEach((child) => {
    const cleanChild = sanitizeNode(child, doc)
    if (cleanChild) clean.appendChild(cleanChild)
  })
  return clean
}

export function sanitizeRichText(value: string) {
  const raw = value || ''
  if (!looksLikeHtml(raw)) {
    const out = document.implementation.createHTMLDocument('')
    out.body.appendChild(enrichText(raw, out))
    return out.body.innerHTML.replace(/\r?\n/g, '<br>')
  }

  const parser = new DOMParser()
  const doc = parser.parseFromString(raw, 'text/html')
  const out = document.implementation.createHTMLDocument('')
  doc.body.childNodes.forEach((child) => {
    const clean = sanitizeNode(child, out)
    if (clean) out.body.appendChild(clean)
  })
  return out.body.innerHTML
}
