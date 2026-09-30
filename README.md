# HALCON

Sistema de ubicación en tiempo real con backend Go, Fiber v3 y Goravel, PostgreSQL y sesiones Redis. Cada usuario obtiene un halcón personal al iniciar sesión y puede compartir su ubicación con otro usuario activo. Los moderadores mantienen la gestión de sus dispositivos y su seguimiento en vivo.

El cliente React está en [`react/`](react/README.md), como proyecto independiente dentro del repositorio. Las vistas tradicionales siguen disponibles en el backend.

## Arranque local

Requisitos: Go 1.26, PostgreSQL, Redis y Node.js 22.12 o posterior para React.

```bash
cp .env.example .env
```

Configura `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME` y `DB_PASSWORD` para una base PostgreSQL existente. Ajusta Redis si no usa localhost:6379. El backend lee `.env` y también admite variables exportadas. Conserva `APP_ENV=local` durante desarrollo; usa `production` y HTTPS al publicar.

```bash
go mod download
go run . artisan key:generate
go run . artisan migrate
go run .
```

Abre http://localhost:3300 y crea una cuenta. No existe una contraseña de administrador predeterminada. Para crear un administrador inicial de forma explícita, configura `BOOTSTRAP_ADMIN_EMAIL` y `BOOTSTRAP_ADMIN_PASSWORD` (al menos 12 caracteres), arranca el backend y retira esas variables después.

Para React, desde otra terminal:

```bash
cd react
npm ci
VITE_BACKEND_PUBLIC_URL=http://127.0.0.1:3300 npm run dev
```

Abre http://127.0.0.1:5173. [La documentación del cliente](react/README.md) describe el proxy, compilación y despliegue bajo un mismo origen público para cookies y WebSocket.

## Verificación

```bash
go test ./...
go vet ./...
node --test tests/frontend/personal-tracking.test.cjs
cd react
npm run build
```

Las pruebas de PostgreSQL y navegador son optativas, usan esquemas temporales y requieren servicios locales. Sus comandos y resultados están en [el plan](PLAN_ACTUALIZACION.md), [el informe de seguridad y UX](SEGURIDAD_UX.md) y [la documentación de React](react/README.md).

## Uso y límites

En el perfil, guarda el correo del destinatario y activa el GPS. El destinatario abre su propio perfil para ver la ubicación. Detener la transmisión conserva la última posición; retirar acceso revoca su visibilidad. El navegador necesita permiso y HTTPS fuera de localhost, y la página debe permanecer abierta. Esta versión mantiene un hub por proceso: utiliza un único proceso de aplicación para la coordinación inmediata de WebSockets.
