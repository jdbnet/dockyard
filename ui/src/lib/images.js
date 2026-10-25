export function imageLabel(img) {
  if (!img) return ''
  if (img.repo_tags?.length) return img.repo_tags.join(', ')
  return img.short_id || img.id || 'image'
}

export function fmtSize(b) {
  if (b == null) return ''
  if (b < 1024) return `${b} B`
  const units = ['KB', 'MB', 'GB', 'TB', 'PB']
  let i = -1
  let size = b
  do { size /= 1024; i++ } while (size >= 1024 && i < units.length - 1)
  return `${size.toFixed(1)} ${units[i]}`
}

export function fmtBytes(b) {
  if (b == null || b === 0) return '-'
  return fmtSize(b)
}

function previousImageLine(img) {
  const label = imageLabel(img)
  const size = img.size ? ` (${fmtSize(img.size)})` : ''
  return `${label}${size}`
}

export async function promptRemovePreviousImage(img) {
  if (!img?.id) return false
  return confirm(`Remove previous image ${previousImageLine(img)}?`)
}

/** Deduplicate by image id; returns true if the user confirmed removal. */
export async function promptRemovePreviousImages(images) {
  const unique = []
  const seen = new Set()
  for (const img of images || []) {
    if (img?.id && !seen.has(img.id)) {
      seen.add(img.id)
      unique.push(img)
    }
  }
  if (!unique.length) return false
  const lines = unique.map(previousImageLine)
  const msg = unique.length === 1
    ? `Remove previous image ${lines[0]}?`
    : `Remove ${unique.length} previous image(s)?\n\n${lines.join('\n')}`
  return confirm(msg)
}
