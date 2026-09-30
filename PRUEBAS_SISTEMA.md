# Validación del sistema HALCON — 2026-10-01

Backend Go, Fiber v3 y Goravel; cliente React y vistas HTML existentes. Los datos de pruebas se crean en esquemas desechables de PostgreSQL. Cada fixture comprueba `current_schema()` antes de migrar y elimina su esquema al terminar. No se insertaron los 1000 usuarios en la base de datos de uso habitual.

## Matriz de funcionalidades

| Funcionalidad | Verificación automatizada |
|---|---|
| Registro y login HTML/API; rol público forzado; halcón personal único | `tracking_integration_test.go`, `thousand_users_test.go` |
| Datos del perfil, normalización de correo y cambio de contraseña | `validation_test.go`, `system_functionality_test.go`, simulación de 1000 usuarios |
| Compartir por correo/ID; retirar acceso; rechazar destinatario propio/inexistente/inactivo | Integración funcional, integración de tracking y carga |
| Permisos de usuario, moderador y administrador; desactivación y cambio de rol | Pruebas de middleware, integración funcional y carga |
| CRUD administrativo de usuarios y dispositivos; eliminación e integridad de relaciones | Integración funcional y carga |
| CRUD del moderador, asignación, historial y aislamiento entre moderadores | Integración funcional, carga y Playwright |
| Paginación y búsqueda de usuarios fuera de la primera página | Integración funcional, carga, Vitest y Playwright con 120 usuarios |
| Asignaciones concurrentes y un solo historial activo | Integración funcional con ocho asignaciones concurrentes |
| Coordenadas finitas y límites geográficos; ubicación válida `(0,0)` | Pruebas unitarias de servicios, parser WebSocket y frontend; integración |
| GPS personal, token de dispositivo, identidad determinada por servidor | Integración de tracking y carga |
| Rechazo de origen ajeno, sesión expirada, CSRF ausente y recursos ajenos | Pruebas unitarias, integración y carga |
| Revalidación de permisos durante la transmisión y retirada del mapa | Integración, carga y pruebas de hooks y mapas |
| Cola saturada y productor reemplazado | Pruebas unitarias del hub y pruebas con detector de carreras |
| JSON sin contraseñas ni tokens | Pruebas de modelos y DTO públicos |
| Activación explícita del GPS, permisos denegados, reconexión y limpieza | Vitest y pruebas Node del cliente HTML |
| Errores HTTP/red, doble envío, conservación del formulario y texto seguro | Vitest y Playwright |
| Diseño en 320, 390, 768 y 1440 píxeles; teclado, movimiento reducido y accesibilidad | Playwright y axe en las pantallas de recorrido React/HTML |
| Migración, rollback y recuperación de halcones personales | Integración de tracking con PostgreSQL real |

Los handlers con transacciones y permisos se verifican mediante peticiones HTTP reales y PostgreSQL, además de las pruebas unitarias de validación. Esta matriz describe escenarios comprobados; no representa cobertura del 100 % de todas las líneas ni de todos los fallos posibles de infraestructura.

Resultados adicionales: `go test ./...`, `go test -race ./app/...` y `go vet ./...` aprobados; 39 pruebas Vitest y tres escenarios Playwright aprobados; compilación TypeScript/Vite aprobada. El parser de coordenadas superó 11 156 ejecuciones generadas mediante fuzzing en una ventana de diez segundos. Las auditorías axe no detectaron infracciones en las pantallas de recorrido examinadas.

## Simulación de 1000 usuarios

Resultado medido: **PASS**, sin fallos, en **225,56 segundos**. El archivo [tests/reports/load-1000-20261001.json](tests/reports/load-1000-20261001.json) conserva los contadores originales.

- 1000 registros y logins reales, con intentos de obtener el rol administrador durante el registro.
- Distribución posterior controlada: 980 usuarios, 18 moderadores y 2 administradores.
- 32 trabajadores concurrentes: perfil, compartir/revocar, GPS, operaciones según rol y logout.
- 30 254 solicitudes HTTP y 14 968 comprobaciones de rechazo de acceso o entrada inválida.
- 1000 espectadores WebSocket abiertos simultáneamente; ventana de lectura de seis segundos, con comprobación de que una coordenada solo llega a su propietario, destinatario y administradores.
- Latencia HTTP p50: 43,80 ms; p95: 922,31 ms; p99: 1416,23 ms. Incluye registro/login con bcrypt; no mide la latencia GPS.
- Cierre de sesión y rechazo posterior de HTTP/WebSocket comprobados para cada usuario.

El administrador conserva su visibilidad global cuando se retira un destinatario: la prueba comprueba esta regla de autorización. Los usuarios ordinarios pierden el acceso. La simulación no equivale a 1000 emisores GPS enviando continuamente ni a una prueba prolongada de producción. El hub sigue siendo local al proceso; varias réplicas necesitan un bus compartido para distribuir eventos.

## Errores corregidos

- Validaciones inconsistentes de nombre, correo, contraseña y rol entre creación, perfil y administración. El límite superior de contraseña respeta los 72 bytes de bcrypt.
- Eliminación de usuarios que omitía el modelo en una actualización e ignoraba errores de transacción; limpieza de relaciones de moderación e historial.
- Middleware de roles que podía aceptar registros inexistentes o inactivos.
- Activación de un dispositivo que inventaba una marca temporal de ubicación antes de recibir GPS.
- Consulta innecesaria de todos los halcones visibles por cada coordenada: ahora se verifica el permiso del recurso específico en SQL. Se añadieron índices de destinatarios y asignaciones activas.
- Consultas de asignaciones por cada fila del listado del moderador; ahora se cargan en bloque.
- Ruta antigua del moderador que intentaba renderizar una plantilla inexistente.
- Selector de asignación limitado a los primeros usuarios; búsqueda accesible por nombre/correo con cancelación e ignorado de respuestas obsoletas.
- Respuestas protegidas sin `Cache-Control: no-store`.
- Errores de JSON/red que interrumpían el frontend o convertían un error temporal del servidor en un falso cierre de sesión.

## Reproducción

Desde la raíz del proyecto:

```bash
go test ./...
go test -race ./app/...
go vet ./...
go test ./app/websockets -fuzz=FuzzDecodeLocation -fuzztime=10s -parallel=4 -timeout=2m
node --test tests/frontend/personal-tracking.test.cjs
```

Usar una base de datos de pruebas dedicada, accesible con las variables habituales `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME` y `DB_PASSWORD`. Ejecutar las suites de base de datos **en procesos separados y secuencialmente**: el descubrimiento de tablas del driver Goravel puede confundir esquemas creados simultáneamente.

```bash
HALCON_INTEGRATION_TEST=1 go test -race . -run '^TestTrackingIntegration$' -count=1 -timeout=3m
HALCON_SYSTEM_TEST=1 go test -race . -run '^TestSystemFunctionalIntegration$' -count=1 -timeout=3m
HALCON_LOAD_TEST=1 HALCON_TEST_REPORT=/tmp/halcon-load-results.json go test . -run '^TestThousandUsersIsolation$' -count=1 -timeout=20m
```

En `react/`:

```bash
npm ci
npm run test:unit
npm run build
```

Para Playwright, compilar el backend y arrancar el fixture aislado en una terminal:

```bash
go build -o /tmp/halcon-browser .
HALCON_BROWSER_FIXTURE=1 HALCON_BROWSER_BINARY=/tmp/halcon-browser HALCON_BROWSER_STOP_FILE=/tmp/halcon-browser-done go test . -run '^TestBrowserFixture$' -count=1 -timeout=12m
```

El marcador debe estar ausente al iniciar. El fixture escucha en 3301 y requiere el Redis configurado. Iniciar React apuntando al fixture y ejecutar las pruebas desde `react/`:

```bash
HALCON_BACKEND_URL=http://127.0.0.1:3301 VITE_BACKEND_PUBLIC_URL=http://127.0.0.1:3301 npm run dev -- --port 5174
HALCON_FRONTEND_URL=http://127.0.0.1:5174 npm run test:e2e
```

Playwright admite `CHROME_PATH` para usar un Chromium ya instalado. Al terminar, crear `/tmp/halcon-browser-done` para que el fixture cierre el backend y elimine el esquema. No ejecutar pruebas con datos reales.

## Referencias técnicas

- [CSRF de Fiber v3](https://docs.gofiber.io/middleware/csrf/) y [sesiones](https://docs.gofiber.io/middleware/session/): vínculo de CSRF con sesión y ciclo de vida.
- [Testing de Goravel](https://docs.goravel.dev/testing/getting-started.html) y [transacciones ORM](https://docs.goravel.dev/orm/getting-started.html).
- [Implementación oficial de bcrypt en Go](https://github.com/golang/crypto/blob/master/bcrypt/bcrypt.go): límite de 72 bytes.
