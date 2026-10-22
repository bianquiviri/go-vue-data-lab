<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Copy } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'

/**
 * Chip compacto para los IDs opacos (`v1.<base64url>`) — feedback de
 * presentación del Issue #20.
 *
 * Los tokens cifrados ocupan ~64 caracteres y ensucian la tabla, pero la columna
 * NO se puede eliminar: el backend rechaza con 400 los UUID crudos, así que el
 * token es la única referencia válida de un registro (detalle, filtros). La
 * solución es ocultarlo tras un chip: se muestra una versión corta legible, el
 * token completo vive en el tooltip nativo (`title`) y un botón lo copia.
 *
 * ⚠️ El token NUNCA se parsea ni transforma: lo que se copia al portapapeles y
 * lo que aparece en el `title` es EXACTAMENTE el string recibido del API. Solo
 * se acorta la PRESENTACIÓN (`shortId`).
 */
const props = withDefaults(defineProps<{ id: string; class?: string; shortLength?: number }>(), {
  shortLength: 10,
})

const { t } = useI18n()

const copied = ref(false)
let resetTimer: ReturnType<typeof setTimeout> | null = null

/** Presentación acortada; `id` intacto en el `title` y el portapapeles. */
const shortId = computed(() => {
  const value = props.id ?? ''
  if (value.length <= props.shortLength) return value
  return `${value.slice(0, props.shortLength)}…`
})

function onCopied(): void {
  copied.value = true
  if (resetTimer !== null) clearTimeout(resetTimer)
  resetTimer = setTimeout(() => {
    copied.value = false
  }, 2000)
}

async function copy(): Promise<void> {
  // Navegadores sin Clipboard API (o contexto no seguro): no romper la vista.
  const clipboard = typeof navigator !== 'undefined' ? navigator.clipboard : undefined
  if (!clipboard || typeof clipboard.writeText !== 'function') return
  try {
    await clipboard.writeText(props.id)
    onCopied()
  } catch {
    // Permiso denegado o fallo del portapapeles: se ignora silenciosamente.
  }
}

onBeforeUnmount(() => {
  if (resetTimer !== null) clearTimeout(resetTimer)
})
</script>

<template>
  <span
    class="bg-muted/60 inline-flex max-w-[10rem] items-center gap-1 rounded-md px-1.5 py-0.5"
    :class="props.class"
    :title="id"
    data-test="id-chip"
  >
    <code class="truncate font-mono text-xs">{{ shortId }}</code>
    <Button
      variant="ghost"
      size="icon-xs"
      type="button"
      :aria-label="copied ? t('data.idCopied') : t('data.idCopy')"
      data-test="id-chip-copy"
      @click="copy"
    >
      <Check v-if="copied" class="text-primary" />
      <Copy v-else />
      <span class="sr-only">{{ copied ? t('data.idCopied') : t('data.idCopy') }}</span>
    </Button>
  </span>
</template>
