<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  getLoggingSettings, getLoggingStatus, getNotificationSettings, getSettings,
  saveLoggingSettings, saveNotificationSettings, saveSettings, testLoggingSettings, testNotificationSettings,
} from '@/api/client'
import { errorMessage } from '@/lib/errors'

const form = ref({
  enabled: false,
  url: '',
  type: 'gologs',
  auth: 'none',
  username: '',
  password: '',
  token: '',
  collect_by_default: true,
})
const passwordSet = ref(false)
const tokenSet = ref(false)
const status = ref({
  state: 'disabled',
  error_count: 0,
  buffered_bytes: 0,
  dropped: 0,
  followed: 0,
  not_collectable: [],
})
const general = ref({
  docker_socket: '',
  stacks_dir: '',
  web_enabled: false,
  web_port: 8080,
  web_bind: '127.0.0.1',
  auth_username: '',
  auth_password: '',
  stats_interval: '5s',
  stats_buffer_size: 60,
  events_fallback_refresh: '30s',
  restart_threshold: 3,
  restart_window: '5m',
  tui_refresh_rate: '100ms',
  updates_interval: '1h',
})
const authPasswordSet = ref(false)
const savingGeneral = ref(false)
const generalMessage = ref('')
const generalError = ref(false)
const saving = ref(false)
const testing = ref(false)
const message = ref('')
const messageError = ref(false)
const notify = ref({
  enabled: false,
  public_url: '',
  schedule: { mode: 'instant', interval: '6h', at: '09:00' },
  webhooks: [],
})
const notifyStatus = ref({ last_error: '', webhook_errors: {} })
const notifySaving = ref(false)
const notifyTesting = ref('')
const notifyMessage = ref('')
const notifyMessageError = ref(false)
let timer

const showUser = computed(() => form.value.auth === 'basic')
const showToken = computed(() => form.value.auth === 'bearer')

onMounted(async () => {
  await loadGeneral()
  await load()
  await loadNotifications()
  await refreshStatus()
  timer = setInterval(refreshStatus, 3000)
})

onUnmounted(() => {
  clearInterval(timer)
})

function applySettings(data) {
  form.value = {
    enabled: data.enabled,
    url: data.url || '',
    type: data.type || 'gologs',
    auth: data.auth || 'none',
    username: data.username || '',
    password: '',
    token: '',
    collect_by_default: data.collect_by_default,
  }
  passwordSet.value = !!data.password_set
  tokenSet.value = !!data.token_set
}

function applyGeneral(data) {
  general.value = {
    docker_socket: data.docker_socket || '',
    stacks_dir: data.stacks_dir || '',
    web_enabled: !!data.web_enabled,
    web_port: data.web_port || 8080,
    web_bind: data.web_bind || '',
    auth_username: data.auth_username || '',
    auth_password: '',
    stats_interval: data.stats_interval || '5s',
    stats_buffer_size: data.stats_buffer_size || 60,
    events_fallback_refresh: data.events_fallback_refresh || '30s',
    restart_threshold: data.restart_threshold || 3,
    restart_window: data.restart_window || '5m',
    tui_refresh_rate: data.tui_refresh_rate || '100ms',
    updates_interval: data.updates_interval || '1h',
  }
  authPasswordSet.value = !!data.auth_password_set
}

async function loadGeneral() {
  try {
    applyGeneral(await getSettings())
  } catch (err) {
    setGeneralMessage(errorMessage(err, 'Failed to load settings'), true)
  }
}

async function saveGeneral() {
  savingGeneral.value = true
  setGeneralMessage('', false)
  try {
    applyGeneral(await saveSettings(general.value))
    setGeneralMessage('Settings saved to config.yaml. Authentication applies now. The Docker socket, web address, stacks directory, and intervals apply after a restart.', false)
  } catch (err) {
    setGeneralMessage(errorMessage(err, 'Failed to save settings'), true)
  } finally {
    savingGeneral.value = false
  }
}

function setGeneralMessage(text, isError) {
  generalMessage.value = text
  generalError.value = isError
}

function applyNotifications(data) {
  notify.value = {
    enabled: !!data.enabled,
    public_url: data.public_url || '',
    schedule: {
      mode: data.schedule?.mode || 'instant',
      interval: data.schedule?.interval || '6h',
      at: data.schedule?.at || '09:00',
    },
    webhooks: (data.webhooks || []).map((wh) => ({
      id: wh.id,
      name: wh.name,
      type: wh.type || 'custom',
      url: '',
      auth: wh.auth || 'none',
      username: wh.username || '',
      token: '',
      enabled: !!wh.enabled,
      url_set: !!wh.url_set,
      token_set: !!wh.token_set,
      message_template: wh.message_template || '',
      title_template: wh.title_template || '',
      custom_fields: (wh.custom_fields || []).map((f) => ({ key: f.key || '', value: f.value || '' })),
    })),
  }
  notifyStatus.value = data.status || { last_error: '', webhook_errors: {} }
}

async function loadNotifications() {
  try {
    applyNotifications(await getNotificationSettings())
  } catch (err) {
    setNotifyMessage(errorMessage(err, 'Failed to load notification settings'), true)
  }
}

const templateHelp = '{{count}} {{scanned_at}} {{summary}} {{summary_plain}} {{summary_br}} {{summary_oneline}} {{link}} {{names}} {{more}}'

function addWebhook() {
  notify.value.webhooks.push({
    id: '',
    name: '',
    type: 'discord',
    url: '',
    auth: 'none',
    username: '',
    token: '',
    enabled: true,
    url_set: false,
    token_set: false,
    message_template: '',
    title_template: '',
    custom_fields: [],
  })
}

function addCustomField(wh) {
  if (!wh.custom_fields) wh.custom_fields = []
  wh.custom_fields.push({ key: '', value: '' })
}

function removeCustomField(wh, index) {
  wh.custom_fields.splice(index, 1)
}

function removeWebhook(index) {
  notify.value.webhooks.splice(index, 1)
}

function notifyPayload() {
  return {
    enabled: notify.value.enabled,
    public_url: notify.value.public_url,
    schedule: notify.value.schedule,
    webhooks: notify.value.webhooks.map((wh) => ({
      id: wh.id,
      name: wh.name,
      type: wh.type,
      url: wh.url,
      auth: wh.auth,
      username: wh.username,
      token: wh.token,
      enabled: wh.enabled,
      message_template: wh.message_template,
      title_template: wh.title_template,
      custom_fields: wh.custom_fields || [],
    })),
  }
}

async function saveNotifications() {
  notifySaving.value = true
  setNotifyMessage('', false)
  try {
    applyNotifications(await saveNotificationSettings(notifyPayload()))
    setNotifyMessage('Notification settings saved.', false)
  } catch (err) {
    setNotifyMessage(errorMessage(err, 'Failed to save notification settings'), true)
  } finally {
    notifySaving.value = false
  }
}

async function testWebhook(wh) {
  notifyTesting.value = wh.id || wh.name
  setNotifyMessage('', false)
  try {
    await testNotificationSettings({
      webhook_id: wh.id,
      webhook: {
        id: wh.id,
        name: wh.name,
        type: wh.type,
        url: wh.url,
        auth: wh.auth,
        username: wh.username,
        token: wh.token,
        enabled: true,
        message_template: wh.message_template,
        title_template: wh.title_template,
        custom_fields: wh.custom_fields || [],
      },
    })
    setNotifyMessage('Test notification sent.', false)
  } catch (err) {
    setNotifyMessage(errorMessage(err, 'Test failed'), true)
  } finally {
    notifyTesting.value = ''
  }
}

function setNotifyMessage(text, isError) {
  notifyMessage.value = text
  notifyMessageError.value = isError
}

const showNotifyUser = (wh) => wh.auth === 'basic'
const showNotifyToken = (wh) => wh.auth === 'bearer'

async function load() {
  try {
    applySettings(await getLoggingSettings())
  } catch (err) {
    setMessage(errorMessage(err, 'Failed to load logging settings'), true)
  }
}

async function refreshStatus() {
  try {
    const data = await getLoggingStatus()
    data.not_collectable = data.not_collectable || []
    status.value = data
  } catch {
    // keep the last status when the request fails
  }
}

function payload() {
  return {
    enabled: form.value.enabled,
    url: form.value.url.trim(),
    type: form.value.type,
    auth: form.value.auth,
    username: form.value.username,
    password: form.value.password,
    token: form.value.token,
    collect_by_default: form.value.collect_by_default,
  }
}

async function save() {
  saving.value = true
  setMessage('', false)
  try {
    applySettings(await saveLoggingSettings(payload()))
    setMessage('Logging settings saved.', false)
    await refreshStatus()
  } catch (err) {
    setMessage(errorMessage(err, 'Failed to save logging settings'), true)
  } finally {
    saving.value = false
  }
}

async function testConnection() {
  testing.value = true
  setMessage('', false)
  try {
    await testLoggingSettings(payload())
    setMessage('Test line accepted.', false)
  } catch (err) {
    setMessage(errorMessage(err, 'Test connection failed'), true)
  } finally {
    testing.value = false
  }
}

function setMessage(text, isError) {
  message.value = text
  messageError.value = isError
}

function formatWhen(value) {
  if (!value || String(value).startsWith('0001-')) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return date.toLocaleString('en-GB')
}

function formatBytes(value) {
  const n = Number(value) || 0
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB']
  let size = n / 1024
  let unit = units[0]
  for (let i = 0; i < units.length; i += 1) {
    unit = units[i]
    if (size < 1024 || i === units.length - 1) break
    size /= 1024
  }
  return `${size.toFixed(1)} ${unit}`
}

const stateLabel = computed(() => {
  if (status.value.state === 'paused_auth') return 'Paused'
  if (status.value.state === 'running') return 'Running'
  return 'Off'
})
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-4">
    <div class="flex flex-wrap gap-2 text-sm">
      <RouterLink to="/settings" class="btn-secondary">General</RouterLink>
      <RouterLink to="/settings/api-keys" class="btn-secondary">API keys</RouterLink>
      <RouterLink to="/docs" class="btn-secondary">API docs</RouterLink>
    </div>
    <section class="card space-y-4">
      <div>
        <h1 class="text-lg font-semibold text-heading">General</h1>
        <p class="mt-1 text-sm text-muted">
          Saved to config.yaml. Environment variables still override these values when Dockyard starts.
        </p>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted" for="docker-socket">Docker socket</label>
          <input id="docker-socket" v-model="general.docker_socket" type="text" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="stacks-dir">Stacks directory</label>
          <input id="stacks-dir" v-model="general.stacks_dir" type="text" class="input-field" />
        </div>
      </div>

      <div class="grid gap-4 sm:grid-cols-3">
        <label class="flex items-center gap-2 text-sm sm:col-span-3">
          <input v-model="general.web_enabled" type="checkbox" class="h-4 w-4 accent-[#1ebe8a]" />
          Enable the web UI on startup
        </label>
        <div>
          <label class="mb-1 block text-sm text-muted" for="web-bind">Bind address</label>
          <input id="web-bind" v-model="general.web_bind" type="text" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="web-port">Port</label>
          <input id="web-port" v-model.number="general.web_port" type="number" min="1" max="65535" class="input-field" />
        </div>
      </div>
      <p class="text-sm text-muted">The Docker socket, stacks directory, and web address apply after a restart.</p>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted" for="auth-user">Login username</label>
          <input id="auth-user" v-model="general.auth_username" type="text" autocomplete="off" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="auth-pass">Login password</label>
          <input
            id="auth-pass"
            v-model="general.auth_password"
            type="password"
            autocomplete="new-password"
            class="input-field"
            :placeholder="authPasswordSet ? 'Saved' : ''"
          />
        </div>
      </div>
      <p class="text-sm text-muted">Leave the username blank to disable login. Leave the password blank to keep the saved one. Login changes apply immediately.</p>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted" for="stats-interval">Stats interval</label>
          <input id="stats-interval" v-model="general.stats_interval" type="text" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="stats-buffer">Stats buffer size</label>
          <input id="stats-buffer" v-model.number="general.stats_buffer_size" type="number" min="1" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="events-refresh">Event refresh</label>
          <input id="events-refresh" v-model="general.events_fallback_refresh" type="text" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="updates-interval">Update scan interval</label>
          <input id="updates-interval" v-model="general.updates_interval" type="text" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="restart-threshold">Restart threshold</label>
          <input id="restart-threshold" v-model.number="general.restart_threshold" type="number" min="1" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="restart-window">Restart window</label>
          <input id="restart-window" v-model="general.restart_window" type="text" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="tui-refresh">TUI refresh</label>
          <input id="tui-refresh" v-model="general.tui_refresh_rate" type="text" class="input-field" />
        </div>
      </div>
      <p class="text-sm text-muted">Intervals apply after a restart. The restart threshold applies immediately. Durations use values such as 100ms, 5s, 5m, or 1h.</p>

      <button type="button" class="btn-primary" :disabled="savingGeneral" @click="saveGeneral">
        {{ savingGeneral ? 'Saving…' : 'Save general settings' }}
      </button>
      <p v-if="generalMessage" class="text-sm" :class="generalError ? 'text-danger' : 'text-accent'">{{ generalMessage }}</p>
    </section>

    <section class="card space-y-4">
      <div>
        <h1 class="text-lg font-semibold text-heading">Logging</h1>
        <p class="mt-1 text-sm text-muted">
          Ship container stdout and stderr to GoLogs or Loki.
        </p>
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input v-model="form.enabled" type="checkbox" class="h-4 w-4 accent-[#1ebe8a]" />
        Enabled
      </label>

      <div>
        <label class="mb-1 block text-sm text-muted" for="log-url">Destination URL</label>
        <input id="log-url" v-model="form.url" type="url" class="input-field" placeholder="http://127.0.0.1:3100/loki/api/v1/push" />
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted" for="log-type">Type</label>
          <select id="log-type" v-model="form.type" class="input-field">
            <option value="gologs">GoLogs</option>
            <option value="loki">Loki</option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="log-auth">Authentication</label>
          <select id="log-auth" v-model="form.auth" class="input-field">
            <option value="none">None</option>
            <option value="basic">Basic</option>
            <option value="bearer">Bearer</option>
          </select>
        </div>
      </div>

      <div v-if="showUser" class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted" for="log-user">Username</label>
          <input id="log-user" v-model="form.username" type="text" autocomplete="off" class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="log-pass">Password</label>
          <input
            id="log-pass"
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            class="input-field"
            :placeholder="passwordSet ? 'Saved' : ''"
          />
        </div>
      </div>

      <div v-if="showToken">
        <label class="mb-1 block text-sm text-muted" for="log-token">Bearer token</label>
        <input
          id="log-token"
          v-model="form.token"
          type="password"
          autocomplete="new-password"
          class="input-field"
          :placeholder="tokenSet ? 'Saved' : ''"
        />
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input v-model="form.collect_by_default" type="checkbox" class="h-4 w-4 accent-[#1ebe8a]" />
        Collect logs by default
      </label>

      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" :disabled="saving" @click="save">
          {{ saving ? 'Saving…' : 'Save' }}
        </button>
        <button type="button" class="btn-secondary" :disabled="testing" @click="testConnection">
          {{ testing ? 'Testing…' : 'Test connection' }}
        </button>
      </div>

      <p v-if="message" class="text-sm" :class="messageError ? 'text-danger' : 'text-accent'">{{ message }}</p>
    </section>

    <section class="card space-y-4">
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-lg font-semibold text-heading">Shipping</h2>
        <span class="text-sm" :class="status.state === 'paused_auth' ? 'text-danger' : 'text-muted'">{{ stateLabel }}</span>
      </div>

      <div
        v-if="status.state === 'paused_auth'"
        class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-danger"
      >
        Authentication failed ({{ status.pause_code }}). Shipping is paused until you save settings.
      </div>

      <dl class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
        <div>
          <dt class="text-muted">Last successful push</dt>
          <dd class="text-heading">{{ formatWhen(status.last_success) }}</dd>
        </div>
        <div>
          <dt class="text-muted">Errors</dt>
          <dd class="text-heading">{{ status.error_count }}</dd>
        </div>
        <div>
          <dt class="text-muted">Buffered</dt>
          <dd class="text-heading">{{ formatBytes(status.buffered_bytes) }}</dd>
        </div>
        <div>
          <dt class="text-muted">Followed</dt>
          <dd class="text-heading">{{ status.followed }}</dd>
        </div>
        <div>
          <dt class="text-muted">Dropped batches</dt>
          <dd class="text-heading">{{ status.dropped }}</dd>
        </div>
      </dl>
      <p v-if="status.last_error && status.state !== 'paused_auth'" class="text-sm text-danger">{{ status.last_error }}</p>

      <div v-if="status.not_collectable.length">
        <h3 class="mb-2 text-sm font-medium text-heading">Not collectable</h3>
        <ul class="space-y-1 text-sm text-muted">
          <li v-for="item in status.not_collectable" :key="item.id">
            <span class="text-heading">{{ item.name || item.id }}</span>
            <span> · {{ item.reason }}</span>
          </li>
        </ul>
      </div>
    </section>

    <section class="card space-y-4">
      <div>
        <h1 class="text-lg font-semibold text-heading">Update notifications</h1>
        <p class="mt-1 text-sm text-muted">
          Webhooks when container image updates are found. Scanning still uses the updates interval above; this controls when alerts are sent.
        </p>
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input v-model="notify.enabled" type="checkbox" class="h-4 w-4 accent-[#1ebe8a]" />
        Enabled
      </label>

      <div>
        <label class="mb-1 block text-sm text-muted" for="notify-public-url">Public URL (optional)</label>
        <input id="notify-public-url" v-model="notify.public_url" type="url" class="input-field" placeholder="https://dockyard.example" />
        <p class="mt-1 text-xs text-muted">Used for “Open updates” links in Discord, Slack, and Teams messages.</p>
      </div>

      <div class="grid gap-4 sm:grid-cols-3">
        <div>
          <label class="mb-1 block text-sm text-muted" for="notify-mode">Schedule</label>
          <select id="notify-mode" v-model="notify.schedule.mode" class="input-field">
            <option value="instant">Instant (when the pending set changes)</option>
            <option value="interval">Interval</option>
            <option value="daily">Daily</option>
          </select>
        </div>
        <div v-if="notify.schedule.mode === 'interval'">
          <label class="mb-1 block text-sm text-muted" for="notify-interval">Every</label>
          <select id="notify-interval" v-model="notify.schedule.interval" class="input-field">
            <option value="3h">3 hours</option>
            <option value="6h">6 hours</option>
            <option value="12h">12 hours</option>
            <option value="24h">24 hours</option>
          </select>
        </div>
        <div v-if="notify.schedule.mode === 'daily'">
          <label class="mb-1 block text-sm text-muted" for="notify-at">At (server local time)</label>
          <input id="notify-at" v-model="notify.schedule.at" type="time" class="input-field" />
        </div>
      </div>

      <div class="space-y-4">
        <div class="flex items-center justify-between gap-3">
          <h2 class="text-sm font-medium text-heading">Webhooks</h2>
          <button type="button" class="btn-secondary" @click="addWebhook">Add webhook</button>
        </div>
        <p v-if="!notify.webhooks.length" class="text-sm text-muted">No webhooks configured.</p>
        <div v-for="(wh, index) in notify.webhooks" :key="wh.id || index" class="rounded-lg border border-default p-4 space-y-3">
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="mb-1 block text-sm text-muted">Name</label>
              <input v-model="wh.name" type="text" class="input-field" />
            </div>
            <div>
              <label class="mb-1 block text-sm text-muted">Type</label>
              <select v-model="wh.type" class="input-field">
                <option value="discord">Discord</option>
                <option value="slack">Slack</option>
                <option value="teams">Teams (Workflows)</option>
                <option value="custom">Custom</option>
              </select>
            </div>
            <div v-if="wh.type !== 'custom'" class="sm:col-span-2">
              <label class="mb-1 block text-sm text-muted">Message template</label>
              <textarea
                v-model="wh.message_template"
                rows="4"
                class="input-field font-mono text-xs"
                :placeholder="`Default: {{count}} container image update(s) + {{summary}}. Variables: ${templateHelp}`"
              />
            </div>
            <div v-if="wh.type === 'discord' || wh.type === 'teams'" class="sm:col-span-2">
              <label class="mb-1 block text-sm text-muted">Title template</label>
              <input
                v-model="wh.title_template"
                type="text"
                class="input-field font-mono text-sm"
                placeholder="Default: Container image updates"
              />
            </div>
            <div v-if="wh.type === 'custom'" class="sm:col-span-2 space-y-2">
              <div class="flex items-center justify-between gap-2">
                <label class="text-sm text-muted">JSON body fields</label>
                <button type="button" class="btn-secondary text-xs" @click="addCustomField(wh)">Add field</button>
              </div>
              <p class="text-xs text-muted">
                Each row becomes one key in the POST body. Values support templates: {{ templateHelp }}.
                On custom webhooks, <span v-pre>{{summary}}</span> uses &lt;br&gt; line breaks for HTML dashboards;
                use <span v-pre>{{summary_plain}}</span> for newline characters in JSON.
              </p>
              <div v-for="(field, fi) in wh.custom_fields" :key="fi" class="grid gap-2 sm:grid-cols-[1fr_2fr_auto]">
                <input v-model="field.key" type="text" class="input-field font-mono text-sm" placeholder="key" />
                <input v-model="field.value" type="text" class="input-field font-mono text-sm" placeholder="value template" />
                <button type="button" class="btn-ghost" @click="removeCustomField(wh, fi)">Remove</button>
              </div>
            </div>
            <div class="sm:col-span-2">
              <label class="mb-1 block text-sm text-muted">Webhook URL</label>
              <input v-model="wh.url" type="password" autocomplete="new-password" class="input-field" :placeholder="wh.url_set ? 'Saved' : 'https://'" />
            </div>
            <div>
              <label class="mb-1 block text-sm text-muted">Auth</label>
              <select v-model="wh.auth" class="input-field">
                <option value="none">None</option>
                <option value="basic">Basic</option>
                <option value="bearer">Bearer</option>
              </select>
            </div>
            <div v-if="showNotifyUser(wh)">
              <label class="mb-1 block text-sm text-muted">Username</label>
              <input v-model="wh.username" type="text" class="input-field" />
            </div>
            <div v-if="showNotifyToken(wh)" class="sm:col-span-2">
              <label class="mb-1 block text-sm text-muted">Token / password</label>
              <input v-model="wh.token" type="password" autocomplete="new-password" class="input-field" :placeholder="wh.token_set ? 'Saved' : ''" />
            </div>
          </div>
          <label class="flex items-center gap-2 text-sm">
            <input v-model="wh.enabled" type="checkbox" class="h-4 w-4 accent-[#1ebe8a]" />
            Enabled
          </label>
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn-secondary" :disabled="notifyTesting === (wh.id || wh.name)" @click="testWebhook(wh)">
              {{ notifyTesting === (wh.id || wh.name) ? 'Testing…' : 'Test' }}
            </button>
            <button type="button" class="btn-ghost" @click="removeWebhook(index)">Remove</button>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" :disabled="notifySaving" @click="saveNotifications">
          {{ notifySaving ? 'Saving…' : 'Save notifications' }}
        </button>
      </div>
      <p v-if="notifyMessage" class="text-sm" :class="notifyMessageError ? 'text-danger' : 'text-accent'">{{ notifyMessage }}</p>
      <dl v-if="notifyStatus.last_notify_at || notifyStatus.last_error" class="grid grid-cols-2 gap-3 text-sm">
        <div>
          <dt class="text-muted">Last notification</dt>
          <dd class="text-heading">{{ formatWhen(notifyStatus.last_notify_at) }}</dd>
        </div>
        <div v-if="notifyStatus.last_error">
          <dt class="text-muted">Last error</dt>
          <dd class="text-danger">{{ notifyStatus.last_error }}</dd>
        </div>
      </dl>
    </section>
  </div>
</template>
