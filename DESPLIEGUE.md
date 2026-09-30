# Despliegue de HALCON

El backend Go sirve las vistas existentes y el build React en `/app/`. La API y los WebSocket permanecen en `/api`, `/location` y `/dashboard`. El dominio configurado en este entorno es `https://halcon.duohnson.com`; Cloudflare Tunnel lo dirige al puerto local 3300.

## Preparar y comprobar

```bash
go test ./...
go vet ./...
cd react
npm ci
npm run test:unit
npm run build
cd ..
go build -o storage/bin/halcon .
go run . artisan migrate
```

La configuración privada de PostgreSQL y Redis permanece en `.env`, fuera del repositorio. El binario y `react/dist/` tampoco se versionan; se reconstruyen desde el código publicado. La migración de índices es aditiva y fue probada con rollback en una base aislada.

## Servicio de usuario

La unidad [deploy/halcon.service](deploy/halcon.service) usa `%h/HALCON` como directorio de trabajo y `%h/HALCON/storage/bin/halcon` como ejecutable. Ajustar estas rutas si cambia la ubicación del proyecto.

```bash
mkdir -p ~/.config/systemd/user
cp deploy/halcon.service ~/.config/systemd/user/halcon.service
loginctl enable-linger "$USER"
systemctl --user daemon-reload
systemctl --user enable --now halcon.service
systemctl --user status halcon.service
```

Detener el proceso anterior que ocupe 3300 antes de iniciar el servicio. La unidad fija `APP_ENV=production`, cookies Secure/HttpOnly y puerto 3300, y reinicia el proceso si falla. Usar HTTPS para iniciar sesión; el cambio de nombre de cookie desde el modo local requiere volver a autenticarse. No desactivar Secure para permitir HTTP público.

El servicio no ejecuta migraciones automáticamente. Para actualizar una instalación existente, compilar React y el binario, aplicar migraciones y ejecutar `systemctl --user restart halcon.service`. Un reinicio interrumpe los WebSocket; los clientes pueden reconectar con una sesión vigente. Consultar logs con `journalctl --user -u halcon.service`.

## Verificación

Comprobar `/login`, `/app/`, sus recursos JS/CSS y `/api/session` por HTTPS. Sin sesión, `/api/tracking` y `/location` deben rechazar acceso. La sesión pública debe usar `__Host-session` y el CSRF `__Host-csrf_`, con `Secure`, `HttpOnly` y sin atributo Domain.

Los flujos con GPS, compartir, revocar y operaciones del moderador se prueban mediante el fixture aislado de [PRUEBAS_SISTEMA.md](PRUEBAS_SISTEMA.md). Para probar el build servido por Go, definir `HALCON_FRONTEND_URL=http://127.0.0.1:3301` y `HALCON_FRONTEND_PATH=/app/` al ejecutar `npm run test:e2e`.

Si el nuevo binario no inicia, detener el servicio y arrancar el binario anterior desde la misma raíz del proyecto. No revertir las migraciones de halcones personales sobre una instalación en uso. Los índices nuevos pueden permanecer con la versión anterior.
