<script setup lang="ts">
/**
 * Footer compartido por TODAS las vistas (Issue del usuario, 21-oct):
 * landing (`PublicLayout`), login (`LoginView`) y panel autenticado
 * (`AppLayout`). Un solo componente ⇒ el texto de licencia/autoría vive en
 * un único sitio.
 *
 * Licencia: CC BY-NC-ND 4.0 (la más restrictiva de Creative Commons —
 * atribución, sin comerciales, sin derivadas). Copy en inglés por adelanto
 * del Issue #37.
 */
</script>

<template>
  <footer class="border-t" data-test="app-footer">
    <div
      class="text-muted-foreground mx-auto flex w-full max-w-5xl flex-col gap-1 px-4 py-6 text-sm sm:flex-row sm:items-center sm:justify-between"
    >
      <p data-test="footer-project">
        Go Concurrent Data Processing Lab — synthetic e-commerce data platform.
      </p>
      <p class="flex flex-wrap items-center gap-x-2">
        <span data-test="footer-author">Developed by Jose Antonio Bianco</span>
        <span aria-hidden="true">·</span>
        <span data-test="footer-license">
          Licensed under
          <a
            href="https://creativecommons.org/licenses/by-nc-nd/4.0/"
            target="_blank"
            rel="noopener noreferrer"
            class="underline underline-offset-2 hover:text-foreground"
          >
            CC BY-NC-ND 4.0
          </a>
        </span>
      </p>
    </div>
  </footer>
</template>
