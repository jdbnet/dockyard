<script setup>
import { ref, watch, onMounted, onBeforeUnmount, shallowRef } from 'vue'
import { EditorView, lineNumbers, highlightActiveLineGutter, highlightSpecialChars, drawSelection, dropCursor, rectangularSelection, crosshairCursor, highlightActiveLine, keymap } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { yaml } from '@codemirror/lang-yaml'
import { linter, lintGutter, lintKeymap } from '@codemirror/lint'
import { HighlightStyle, syntaxHighlighting, foldGutter, indentOnInput, bracketMatching, foldKeymap } from '@codemirror/language'
import { tags } from '@lezer/highlight'
import { history, defaultKeymap, historyKeymap } from '@codemirror/commands'
import { highlightSelectionMatches, searchKeymap } from '@codemirror/search'
import { closeBrackets, autocompletion, closeBracketsKeymap, completionKeymap } from '@codemirror/autocomplete'
import { load as parseYAML } from 'js-yaml'
import { useThemeStore } from '@/stores/theme'

const dockyardHighlightDark = HighlightStyle.define([
  { tag: tags.propertyName, color: '#1ebe8a' },
  { tag: tags.definition(tags.propertyName), color: '#1ebe8a' },
  { tag: tags.variableName, color: '#e2e8f0' },
  { tag: tags.definition(tags.variableName), color: '#93c5fd' },
  { tag: tags.local(tags.variableName), color: '#93c5fd' },
  { tag: tags.keyword, color: '#c4b5fd' },
  { tag: [tags.atom, tags.bool, tags.url, tags.contentSeparator, tags.labelName], color: '#fbbf24' },
  { tag: [tags.literal, tags.inserted], color: '#fbbf24' },
  { tag: [tags.number, tags.integer, tags.float], color: '#fbbf24' },
  { tag: [tags.string, tags.deleted], color: '#86efac' },
  { tag: [tags.regexp, tags.escape, tags.special(tags.string)], color: '#fca5a5' },
  { tag: [tags.typeName, tags.namespace], color: '#67e8f9' },
  { tag: tags.className, color: '#67e8f9' },
  { tag: [tags.special(tags.variableName), tags.macroName], color: '#f0abfc' },
  { tag: tags.comment, color: '#64748b', fontStyle: 'italic' },
  { tag: tags.meta, color: '#64748b' },
  { tag: tags.heading, fontWeight: 'bold', color: '#e2e8f0' },
  { tag: tags.invalid, color: '#f87171' },
  { tag: tags.punctuation, color: '#94a3b8' },
  { tag: tags.bracket, color: '#94a3b8' },
  { tag: tags.separator, color: '#94a3b8' },
  { tag: tags.processingInstruction, color: '#64748b' },
  { tag: tags.link, color: '#1ebe8a', textDecoration: 'underline' },
  { tag: tags.emphasis, fontStyle: 'italic' },
  { tag: tags.strong, fontWeight: 'bold' },
  { tag: tags.strikethrough, textDecoration: 'line-through' },
])

const dockyardHighlightLight = HighlightStyle.define([
  { tag: tags.propertyName, color: '#19986e' },
  { tag: tags.definition(tags.propertyName), color: '#19986e' },
  { tag: tags.variableName, color: '#1f2328' },
  { tag: tags.definition(tags.variableName), color: '#0550ae' },
  { tag: tags.local(tags.variableName), color: '#0550ae' },
  { tag: tags.keyword, color: '#8250df' },
  { tag: [tags.atom, tags.bool, tags.url, tags.contentSeparator, tags.labelName], color: '#953800' },
  { tag: [tags.literal, tags.inserted], color: '#953800' },
  { tag: [tags.number, tags.integer, tags.float], color: '#953800' },
  { tag: [tags.string, tags.deleted], color: '#0a3069' },
  { tag: [tags.regexp, tags.escape, tags.special(tags.string)], color: '#cf222e' },
  { tag: [tags.typeName, tags.namespace], color: '#0550ae' },
  { tag: tags.className, color: '#0550ae' },
  { tag: [tags.special(tags.variableName), tags.macroName], color: '#8250df' },
  { tag: tags.comment, color: '#656d76', fontStyle: 'italic' },
  { tag: tags.meta, color: '#656d76' },
  { tag: tags.heading, fontWeight: 'bold', color: '#1f2328' },
  { tag: tags.invalid, color: '#cf222e' },
  { tag: tags.punctuation, color: '#57606a' },
  { tag: tags.bracket, color: '#57606a' },
  { tag: tags.separator, color: '#57606a' },
  { tag: tags.processingInstruction, color: '#656d76' },
  { tag: tags.link, color: '#19986e', textDecoration: 'underline' },
  { tag: tags.emphasis, fontStyle: 'italic' },
  { tag: tags.strong, fontWeight: 'bold' },
  { tag: tags.strikethrough, textDecoration: 'line-through' },
])

const props = defineProps({
  modelValue: { type: String, default: '' },
  readOnly: { type: Boolean, default: false },
  minHeight: { type: String, default: '24rem' },
})

const emit = defineEmits(['update:modelValue'])
const theme = useThemeStore()

const host = ref(null)
const view = shallowRef(null)
const readOnlyCompartment = new Compartment()
const highlightCompartment = new Compartment()
const darkThemeCompartment = new Compartment()

function highlightFor(dark) {
  return syntaxHighlighting(dark ? dockyardHighlightDark : dockyardHighlightLight)
}

const editorSetup = [
  lineNumbers(),
  highlightActiveLineGutter(),
  highlightSpecialChars(),
  history(),
  foldGutter(),
  drawSelection(),
  dropCursor(),
  EditorState.allowMultipleSelections.of(true),
  indentOnInput(),
  bracketMatching(),
  closeBrackets(),
  autocompletion(),
  rectangularSelection(),
  crosshairCursor(),
  highlightActiveLine(),
  highlightSelectionMatches(),
  keymap.of([
    ...closeBracketsKeymap,
    ...defaultKeymap,
    ...searchKeymap,
    ...historyKeymap,
    ...foldKeymap,
    ...completionKeymap,
    ...lintKeymap,
  ]),
]

function posFromMark(doc, mark) {
  if (!mark) return 0
  const lines = doc.split('\n')
  let pos = 0
  for (let i = 0; i < mark.line; i++) {
    pos += (lines[i]?.length ?? 0) + 1
  }
  pos += mark.column
  return Math.min(pos, doc.length)
}

function yamlLint(editorView) {
  const diagnostics = []
  const doc = editorView.state.doc.toString()
  if (!doc.trim()) return diagnostics
  try {
    parseYAML(doc)
  } catch (e) {
    const from = posFromMark(doc, e.mark)
    const line = editorView.state.doc.lineAt(from)
    diagnostics.push({
      from: line.from,
      to: line.to,
      severity: 'error',
      message: e.reason || e.message,
    })
  }
  return diagnostics
}

function buildExtensions() {
  return [
    ...editorSetup,
    highlightCompartment.of(highlightFor(theme.dark)),
    darkThemeCompartment.of(EditorView.darkTheme.of(theme.dark)),
    yaml(),
    lintGutter(),
    linter(yamlLint),
    EditorView.lineWrapping,
    EditorView.theme({
      '&': { minHeight: props.minHeight },
      '.cm-scroller': { overflow: 'auto', fontFamily: 'ui-monospace, monospace' },
      '.cm-content': { padding: '0.5rem 0', caretColor: '#1ebe8a' },
      '.cm-cursor, .cm-dropCursor': { borderLeftColor: '#1ebe8a' },
      '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
        backgroundColor: '#1ebe8a33 !important',
      },
      '.cm-foldPlaceholder': { border: 'none' },
      '&.cm-focused': { outline: 'none' },
      '&.cm-focused .cm-matchingBracket': { backgroundColor: '#1ebe8a44' },
      '&.cm-focused .cm-nonmatchingBracket': { backgroundColor: '#f8717144' },
    }),
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        emit('update:modelValue', update.state.doc.toString())
      }
    }),
    readOnlyCompartment.of(EditorState.readOnly.of(props.readOnly)),
  ]
}

onMounted(() => {
  view.value = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions: buildExtensions(),
    }),
    parent: host.value,
  })
})

watch(() => props.modelValue, (value) => {
  if (!view.value) return
  const current = view.value.state.doc.toString()
  if (value !== current) {
    view.value.dispatch({
      changes: { from: 0, to: view.value.state.doc.length, insert: value },
    })
  }
})

watch(() => props.readOnly, (readOnly) => {
  if (!view.value) return
  view.value.dispatch({
    effects: readOnlyCompartment.reconfigure(EditorState.readOnly.of(readOnly)),
  })
})

watch(() => theme.dark, (dark) => {
  if (!view.value) return
  view.value.dispatch({
    effects: [
      highlightCompartment.reconfigure(highlightFor(dark)),
      darkThemeCompartment.reconfigure(EditorView.darkTheme.of(dark)),
    ],
  })
})

onBeforeUnmount(() => {
  view.value?.destroy()
})
</script>

<template>
  <div ref="host" class="compose-editor rounded-lg border border-default overflow-hidden text-sm" />
</template>
