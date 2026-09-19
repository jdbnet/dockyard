export function errorMessage(err, fallback = 'Request failed') {
  return err?.response?.data?.error || err?.message || fallback
}
