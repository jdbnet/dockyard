import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  withCredentials: true,
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && !window.location.pathname.startsWith('/login')) {
      const redirect = encodeURIComponent(window.location.pathname + window.location.search)
      window.location.href = `/login?redirect=${redirect}`
    }
    return Promise.reject(error)
  },
)

export default api

export async function getContainers(flat = false) {
  const url = flat ? '/containers' : '/compose'
  const { data } = await api.get(url)
  return data
}

export async function getContainer(id) {
  const { data } = await api.get(`/containers/${id}`)
  return data
}

export async function getContainerStats(id) {
  const { data } = await api.get(`/containers/${id}/stats`)
  return data
}

export async function containerAction(id, action) {
  await api.post(`/containers/${id}/${action}`)
}

export async function removeContainer(id) {
  await api.delete(`/containers/${id}`)
}

export async function updateContainer(id) {
  await api.post(`/containers/${id}/update`)
}

export async function getStacks() {
  const { data } = await api.get('/stacks')
  return data
}

export async function getStackCompose(name) {
  const { data } = await api.get(`/stacks/${name}/compose`)
  return data
}

export async function saveStackCompose(name, content) {
  await api.put(`/stacks/${name}/compose`, { content })
}

export async function createStack(name, content, start = false) {
  await api.post('/stacks', { name, content, start })
}

export async function stackUp(name) {
  await api.post(`/stacks/${name}/up`)
}

export async function stackDown(name) {
  await api.post(`/stacks/${name}/down`)
}

export async function stackUpdate(name) {
  await api.post(`/stacks/${name}/update`)
}

export async function deleteStack(name) {
  await api.delete(`/stacks/${name}`)
}

export async function getImages() {
  const { data } = await api.get('/images')
  return data
}

export async function getVolumes() {
  const { data } = await api.get('/volumes')
  return data
}

export async function getNetworks() {
  const { data } = await api.get('/networks')
  return data
}

export async function removeImage(id) {
  await api.delete(`/images/${id}`)
}

export async function pruneUnusedImages() {
  const { data } = await api.post('/images/prune')
  return data
}

export async function removeVolume(name) {
  await api.delete(`/volumes/${name}`)
}

export async function removeNetwork(id) {
  await api.delete(`/networks/${id}`)
}

export function wsURL(path) {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}${path}`
}
