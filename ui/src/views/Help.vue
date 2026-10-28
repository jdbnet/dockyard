<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import {
  BookOpen, Rocket, RefreshCw, Tags, Bell, ScrollText, Settings, CircleHelp,
} from '@lucide/vue'

const mdModules = import.meta.glob('../help/*.md', {
  query: '?raw',
  import: 'default',
  eager: true,
})

const pages = [
  { slug: 'overview', title: 'Overview', icon: BookOpen },
  { slug: 'deploying', title: 'Deploying', icon: Rocket },
  { slug: 'updates', title: 'Updates', icon: RefreshCw },
  { slug: 'labels', title: 'Labels', icon: Tags },
  { slug: 'notifications', title: 'Notifications', icon: Bell },
  { slug: 'logging', title: 'Logging', icon: ScrollText },
  { slug: 'settings-and-api', title: 'Settings and API', icon: Settings },
]

const route = useRoute()
const router = useRouter()

const md = new MarkdownIt({ html: false, linkify: true })

const slug = computed(() => {
  const raw = route.params.page
  if (!raw || (Array.isArray(raw) && !raw[0])) return 'overview'
  return Array.isArray(raw) ? raw[0] : raw
})

const currentPage = computed(() => pages.find((p) => p.slug === slug.value))

const unknownPage = computed(() => slug.value !== 'overview' && !currentPage.value)

const source = computed(() => {
  const key = `../help/${slug.value}.md`
  if (mdModules[key]) return mdModules[key]
  return mdModules['../help/overview.md']
})

// Sanitised HTML only - markdown-it escapes raw HTML; DOMPurify is a second layer before v-html.
const html = computed(() => DOMPurify.sanitize(md.render(source.value || '')))

function onContentClick(event) {
  const anchor = event.target.closest('a')
  if (!anchor) return
  const href = anchor.getAttribute('href')
  if (!href || !href.startsWith('/help')) return
  event.preventDefault()
  router.push(href)
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center gap-2 text-heading">
      <CircleHelp class="h-6 w-6 text-accent" aria-hidden="true" />
      <h1 class="text-lg font-semibold">Help</h1>
    </div>

    <div class="grid gap-4 lg:grid-cols-[14rem_minmax(0,1fr)]">
      <nav class="card space-y-1 p-2">
        <RouterLink
          v-for="page in pages"
          :key="page.slug"
          :to="`/help/${page.slug}`"
          class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm"
          :class="slug === page.slug ? 'nav-item-active' : 'nav-item-inactive'"
        >
          <component :is="page.icon" class="h-4 w-4 shrink-0" aria-hidden="true" />
          {{ page.title }}
        </RouterLink>
      </nav>

      <article class="card help-prose" @click="onContentClick" v-html="html" />
    </div>

    <p v-if="unknownPage" class="text-sm text-muted">
      That help page was not found - showing the overview instead.
    </p>
  </div>
</template>
