# HALCON React

Cliente web separado de HALCON, construido con React, TypeScript, Vite y Leaflet. El backend continúa en Go, Fiber v3 y Goravel. Incluye inicio y cierre de sesión, selección del destinatario por correo, revocación, GPS voluntario y mapa en tiempo real. Moderadores y administradores acceden también a la gestión existente mediante enlaces al backend.

## Ejecutar

Usa Node.js 22.12 o posterior y el backend actualizado funcionando en el puerto 3300 con PostgreSQL y Redis configurados.

```bash
cd react
npm ci
npm run dev
```

Abre http://127.0.0.1:5173. Si usas otro puerto del backend:

```bash
HALCON_BACKEND_URL=http://127.0.0.1:3300 VITE_BACKEND_PUBLIC_URL=http://127.0.0.1:3300 npm run dev
```

`HALCON_BACKEND_URL` configura el proxy HTTP/WebSocket del servidor de desarrollo; `VITE_BACKEND_PUBLIC_URL` configura los enlaces a la administración. No deben contener secretos. El proxy conserva Host y Origin para las verificaciones del backend. La sesión usa cookies del servidor y CSRF; no se almacenan credenciales ni tokens de sesión en localStorage.

## Compilar y publicar

```bash
npm run build
```

El resultado queda en `dist/`, con prefijo de recursos `/app/`. El backend actualizado sirve automáticamente ese directorio en `/app/` cuando existe `dist/index.html`. Publica el backend bajo HTTPS; React, la API y WebSocket comparten origen y sesión. Compila antes de arrancar el backend. Los enlaces de administración y registro usan el mismo origen en producción; `VITE_BACKEND_PUBLIC_URL` permite configurar otro y se fija al compilar. `npm run preview` sólo sirve para revisar los archivos estáticos: no configura el proxy al backend.

Este proyecto es una aplicación web adaptable. La página debe permanecer abierta para transmitir GPS; no ofrece ubicación garantizada con el teléfono bloqueado.

## Pruebas reales

`npm run test:e2e` usa Chrome y un backend con usuarios de prueba. Instala Chromium mediante `npx playwright install chromium` o define `CHROME_PATH` apuntando a un Chrome instalado. No lo ejecutes contra una base de producción. Desde la raíz de HALCON, con PostgreSQL de pruebas y Redis local:

```bash
go build -o /tmp/halcon-browser-preview .
HALCON_BROWSER_FIXTURE=1 HALCON_BROWSER_BINARY=/tmp/halcon-browser-preview HALCON_BROWSER_STOP_FILE=/tmp/halcon-browser-done DB_HOST=127.0.0.1 DB_PORT=55439 DB_DATABASE=halcon_validation DB_USERNAME=peter go test . -run TestBrowserFixture -count=1 -v -timeout=12m
```

En otras terminales, desde este directorio:

```bash
HALCON_BACKEND_URL=http://127.0.0.1:3301 VITE_BACKEND_PUBLIC_URL=http://127.0.0.1:3301 npm run dev
CHROME_PATH=/ruta/al/chrome npm run test:e2e
```

Al terminar, `touch /tmp/halcon-browser-done` detiene el fixture y elimina su esquema. Usa una ruta de cierre que todavía no exista. Ejecuta los fixtures de base de datos consecutivamente: el descubrimiento de tablas del driver de Goravel puede detectar tablas en otro esquema temporal activo. Los informes y capturas quedan en `test-results/`. Si el frontend usa otro puerto, define `HALCON_FRONTEND_URL` al ejecutar Playwright.

Validado en Chrome: sesión, GPS simulado, datos maliciosos mostrados como texto, errores del formulario, revocación, teclado, movimiento reducido y anchuras 320/390/768/1440 px. Axe revisa las páginas de seguimiento completas; esa revisión automática no equivale a una certificación WCAG de todo el sistema.

Documentación contrastada: [React useEffect](https://react.dev/reference/react/useEffect), [Vite](https://vite.dev/guide/) y [Leaflet](https://leafletjs.com/reference.html).
