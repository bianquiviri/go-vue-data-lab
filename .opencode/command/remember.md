---
description: Guarda en docs/MEMORY.md lo esencial de la sesión para no perder contexto entre reinicios de opencode.
agent: orchestrator
---

Analiza la conversación actual y **actualiza** `docs/MEMORY.md` conservando lo que ya exista.

Debes dejar registrado, de forma concisa:
1. **Estado actual**: qué se completó en esta sesión (archivos creados/modificados y por qué).
2. **Decisiones tomadas**: arquitectura, convenciones, librerías elegidas.
3. **Problemas / bugs conocidos**: pendientes y bloqueos.
4. **Siguiente paso**: la acción concreta inmediata para retomar el trabajo.

Instrucciones adicionales del usuario (pueden ir vacías): $ARGUMENTS

Reglas:
- Edita `docs/MEMORY.md` (no crees otro archivo).
- Sé breve; no pegues bloques largos de código.
- Si algo contradice una entrada previa, actualízala en lugar de duplicarla.
- Al terminar, confirma con una lista corta de lo que guardaste.
