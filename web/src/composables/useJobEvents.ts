import { onUnmounted, ref, shallowRef, watch } from 'vue'

import { ApiError } from '@/api/client'
import { buildJobEventsUrl, getJobProgress, getJobStatus } from '@/api/jobs'
import { useAuthStore } from '@/stores/auth'
import { type JobProgress, type JobStatus, isTerminal } from '@/types/jobs'

/**
 * Suscripción al stream SSE de progreso de jobs (Issue #21).
 *
 * Ciclo de vida del `EventSource`:
 *  - **Creación**: `connect()` construye la URL con el access token ACTUAL del
 *    store de auth en el query param `token` (codificado) y abre el stream.
 *    `EventSource` no permite cabeceras, de ahí el token por query.
 *  - **Snapshot**: el backend emite un snapshot inmediato al suscribirse. En cada
 *    (re)apertura se refresca además `GET /jobs/status`, porque si el stream se
 *    reconectó mientras un job avanzaba podríamos haber perdido el último frame.
 *  - **Eventos**: cada frame `event: progress` se parsea a `JobProgress`. El
 *    heartbeat `: ping` es un comentario y `EventSource` NO lo emite como evento,
 *    así que no hay nada que filtrar.
 *  - **Cierre**: el backend cierra el stream al terminar el job (tras el evento
 *    terminal). `close()` se llama en `onUnmounted` para no dejar conexiones
 *    abiertas.
 *  - **Sin reconexión automática**: el `EventSource` nativo reconecta SOLO con
 *    la MISMA URL (token viejo) y, tras un job terminal, el backend cierra de
 *    inmediato a cualquier suscriptor nuevo ⇒ un bucle de reconexiones. Por eso
 *    el composable cierra la fuente en cuanto el stream termina o falla: la
 *    reconexión es manual (botón "Reconectar") o la reabre la vista al arrancar
 *    un job nuevo (`connect()`).
 *  - **Sesión**: si la sesión termina (logout) el stream se cierra; si el token
 *    rota (refresh) se reabre con el token vigente para no dejar un token
 *    caducado en la URL.
 *  - **Fallo / token caducado**: el composable marca `disconnected` y la vista
 *    ofrece reconectar (`reconnect`) o resolver la sesión (`authExpired`). El
 *    resto de la vista sigue funcionando: `refreshProgress()` es el fallback
 *    por REST.
 */
export interface UseJobEventsReturn {
  status: ReturnType<typeof ref<JobStatus | null>>
  progress: ReturnType<typeof ref<JobProgress | null>>
  /** El stream está abierto y entregando frames. */
  connected: ReturnType<typeof ref<boolean>>
  /** El stream falló/cerró y no se reintenta automáticamente. */
  disconnected: ReturnType<typeof ref<boolean>>
  /** El fallo parece de sesión (401): el token pudo caducar. */
  authExpired: ReturnType<typeof ref<boolean>>
  connect: () => void
  close: () => void
  reconnect: () => void
  /** Fallback REST cuando no hay SSE: pide el progreso puntual. */
  refreshProgress: () => Promise<void>
  /** Refresca el snapshot completo (estado + progreso) por REST. */
  refreshStatus: () => Promise<void>
}

export function useJobEvents(): UseJobEventsReturn {
  const auth = useAuthStore()

  const status = ref<JobStatus | null>(null)
  const progress = ref<JobProgress | null>(null)
  const connected = ref(false)
  const disconnected = ref(false)
  const authExpired = ref(false)

  // `shallowRef`: el EventSource no necesita reactividad profunda.
  const source = shallowRef<EventSource | null>(null)
  /** Token con el que se abrió el stream actual (para saber si hay que reabrirlo). */
  let currentToken: string | null = null

  function close(): void {
    source.value?.close()
    source.value = null
    currentToken = null
    connected.value = false
  }

  /** Refresca el snapshot completo (estado + progreso) por REST. */
  async function refreshStatus(): Promise<void> {
    try {
      const snapshot = await getJobStatus()
      status.value = snapshot
      if (snapshot.progress) progress.value = snapshot.progress
      authExpired.value = false
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) authExpired.value = true
    }
  }

  /** Fallback REST: el progreso puntual cuando el SSE no está disponible. */
  async function refreshProgress(): Promise<void> {
    try {
      progress.value = await getJobProgress()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) authExpired.value = true
    }
  }

  function onProgress(event: MessageEvent): void {
    try {
      const next = JSON.parse(event.data as string) as JobProgress
      progress.value = next
      // Refleja el frame en el snapshot si ya lo tenemos (evita esperar al
      // siguiente GET /jobs/status para ver el estado terminal).
      if (status.value) {
        status.value = {
          ...status.value,
          status: next.status,
          running: next.status === 'running',
          progress: next,
        }
      }
      // El backend cierra el stream tras el evento terminal; cerramos nosotros
      // para no depender del EOF y para que el `EventSource` no intente
      // reconectar en bucle (el backend cierra de inmediato a cualquier
      // suscriptor de un job ya terminal).
      if (isTerminal(next.status)) close()
    } catch {
      // Frame no parseable: se ignora sin romper el stream.
    }
  }

  function connect(): void {
    const token = auth.accessToken
    if (!token) {
      disconnected.value = true
      return
    }
    // El EventSource nativo no existe en SSR/jsdom sin polyfill.
    if (typeof EventSource === 'undefined') {
      disconnected.value = true
      return
    }

    close()
    disconnected.value = false
    authExpired.value = false
    currentToken = token

    // El backend emite un snapshot inmediato al suscribirse; lo pedimos también
    // por REST para que la UI tenga estado aunque el primer frame tarde.
    void refreshStatus()

    const es = new EventSource(buildJobEventsUrl(token))
    es.onopen = () => {
      connected.value = true
      disconnected.value = false
      // Refresca el snapshot al (re)abrir para no perder el estado acumulado.
      void refreshStatus()
    }
    es.addEventListener('progress', onProgress)
    es.onerror = () => {
      // Ignora los errores de una fuente que ya fue sustituida o cerrada.
      if (source.value !== es) return
      connected.value = false
      // Cerramos la fuente: el `EventSource` reconectaría SOLO con la misma
      // URL (token viejo) y, tras un job terminal, el backend cerraría cada
      // suscriptor de inmediato ⇒ bucle de reconexiones. La reconexión es
      // manual (botón) o la reabre la vista al arrancar un job nuevo.
      close()
      // El backend cierra el stream tras emitir el evento terminal: eso NO
      // es un fallo, es el final normal del job.
      const last = progress.value?.status
      if (last && isTerminal(last)) {
        disconnected.value = false
        return
      }
      disconnected.value = true
      // Comprueba si el fallo se debe a una sesión caducada.
      void refreshStatus()
    }
    source.value = es
  }

  function reconnect(): void {
    connect()
  }

  // La sesión puede terminar (logout) o rotar el token (refresh): el stream
  // debe morir con la sesión y reabrirse con el token vigente.
  watch(
    () => auth.accessToken,
    (token) => {
      if (!token) {
        close()
        disconnected.value = true
        return
      }
      // Token rotado: se reabre el stream con el token vigente para no dejar
      // un token caducado en la URL del `EventSource`.
      if (source.value && token !== currentToken) connect()
    },
  )

  onUnmounted(() => {
    close()
  })

  return {
    status,
    progress,
    connected,
    disconnected,
    authExpired,
    connect,
    close,
    reconnect,
    refreshProgress,
    refreshStatus,
  }
}
