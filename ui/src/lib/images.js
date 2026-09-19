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

export async function promptRemovePreviousImage(img) {
  if (!img?.id) return false
  const label = imageLabel(img)
  const size = img.size ? ` (${fmtSize(img.size)})` : ''
  return confirm(`Remove previous image ${label}${size}?`)
}
