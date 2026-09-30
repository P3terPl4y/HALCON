# Actualización del flujo de HALCON

## Plan y decisiones

1. Revisar modelos, migraciones, autenticación, vistas y WebSockets. La aplicación usa Fiber v3 para HTTP y sesiones, y Goravel para ORM y migraciones; no se introduce otro servidor ni otro framework.
2. Separar el halcón personal del dispositivo creado por un moderador. `owner_id` identifica al emisor personal, `recipient_id` a su destinatario; las asignaciones existentes siguen atendiendo a los dispositivos del moderador.
3. Crear el halcón personal al iniciar sesión o registrarse. Una transacción bloquea la fila del usuario y un índice único evita duplicados incluso con inicios simultáneos.
4. Añadir un formulario protegido por el CSRF existente para seleccionar un destinatario activo por correo. La API conserva compatibilidad con ID; `0` retira el acceso. No se permite compartir consigo mismo.
5. Transmitir GPS con `/location`, autenticado por sesión. El servidor determina el halcón; el navegador no puede escoger otro emisor. Los dispositivos conservan su endpoint con token, verificando ambos IDs de la ruta y la asignación vigente.
6. Mostrar en `/profile` los halcones personales propios, recibidos y los dispositivos del moderador. Los administradores mantienen visibilidad global. El panel de moderación enlaza al mapa.
7. Revalidar acceso al transmitir y al recibir. Enviar coordenadas en vivo y snapshots cada dos segundos para reconciliar asignaciones, últimas posiciones y revocaciones. Serializar las escrituras de cada dashboard, limitar mensajes y proteger la sustitución de conexiones.
8. Validar compilación, JavaScript, migración y reversión, concurrencia, entrega de GPS, permisos y logout en PostgreSQL y WebSockets locales.

## Uso

Iniciar sesión y abrir el perfil. Guardar el correo del destinatario y pulsar **Activar ubicación**. El navegador necesita permiso GPS y HTTPS, salvo en localhost. El destinatario abre su propio perfil para ver el mapa. El moderador conserva la creación, asignación y eliminación de sus dispositivos y puede abrir **Mapa en tiempo real**.

**Detener transmisión** detiene los nuevos puntos; el destinatario conserva la última ubicación conocida. **Retirar acceso** revoca ese acceso. Una nueva pestaña que transmita sustituye a la anterior y esta deja de reconectarse automáticamente.

Los rastros se mantienen únicamente en memoria mientras la página está abierta y se retiran al perder acceso o conexión. Las últimas coordenadas se guardan en PostgreSQL. No se envían contraseñas ni tokens en respuestas JSON.

## Activación y verificación

Ejecutar desde la raíz del proyecto:

```bash
go test ./...
go vet ./...
node --test tests/frontend/personal-tracking.test.cjs
go run . artisan migrate
```

Después iniciar el servidor actualizado con el mecanismo habitual del entorno. La migración añade columnas nullable, claves foráneas, un índice único y precisión de siete decimales para futuras coordenadas. No convierte dispositivos existentes en personales ni puede recuperar precisión perdida previamente.

Prueba de integración opcional, con una base de pruebas cuyo usuario tenga permiso de crear esquemas:

```bash
HALCON_INTEGRATION_TEST=1 DB_HOST=127.0.0.1 DB_PORT=5432 DB_DATABASE=halcon_test DB_USERNAME=halcon_test go test -race . -run TestTrackingIntegration -count=1 -v
```

La prueba crea y elimina su propio esquema y comprueba que las consultas del ORM lo usan antes de ejecutar las migraciones. Incluye creación concurrente, selección y cambio de destinatario, aislamiento de moderadores, precisión GPS, entrega en vivo, rechazo de conexiones anónimas y de otro origen, sustitución de emisor y revocación por logout.

## Límites operativos

- El navegador debe permanecer abierto; este flujo no garantiza GPS en segundo plano con la pantalla bloqueada. Para ello se necesita un cliente móvil apropiado.
- El hub es local al proceso. Con varios procesos, los snapshots de PostgreSQL actualizan los mapas, pero la sustitución inmediata de emisores y los eventos instantáneos requieren coordinación compartida. Para esta versión se recomienda un solo proceso de aplicación.
- El acceso revocado desaparece del mapa en el siguiente snapshot, como máximo aproximadamente dos segundos mientras la conexión y la base funcionen. Los datos que un destinatario ya recibió no pueden retirarse de su dispositivo.
- Las conexiones se cierran tras treinta minutos y sólo pueden reconectar si la sesión sigue vigente; se comprueba también su existencia en el almacén de sesiones. Un emisor sin datos nuevos caduca tras sesenta segundos.
- No se ejecuta `migrate:rollback` mientras se utiliza esta versión: retiraría las columnas que necesita el código. La prueba de reversión se realiza en el esquema temporal.

## Documentación consultada

- [Fiber v3 WebSocket y Locals](https://docs.gofiber.io/recipes/websocket/)
- [WebSocket contrib v3](https://github.com/gofiber/contrib/blob/main/v3/websocket/README.md)
- [Goravel ORM, transacciones y bloqueos](https://docs.goravel.dev/orm/getting-started.html)
- [Goravel migraciones](https://docs.goravel.dev/database/migrations.html)

Además se contrastaron las firmas con el código local de Fiber 3.5.0, websocket 1.2.6 y Goravel 1.18.0 fijado en `go.mod`.

## Estado verificado el 1 de octubre de 2026

La ampliación de seguridad, UX y el cliente separado React están documentados en `SEGURIDAD_UX.md` y `react/README.md`. El backend conserva Go; el cliente React consume la misma sesión y las mismas reglas de visibilidad.

- `go test ./...`, `go vet ./...` y las pruebas de JavaScript finalizaron correctamente.
- La integración con PostgreSQL y WebSockets pasó con `-race`, sin carreras detectadas. Se probaron los formularios con CSRF activado y el migrador completo, incluyendo rollback y recuperación cuando el esquema existe pero su registro quedó pendiente.
- La ejecución inicial del migrador reveló una transacción anidada incompatible con Goravel 1.18. Se corrigió para usar la transacción del framework y se completó el registro sin eliminar datos existentes.
- `20261001000000_personal_halcones` figura como aplicada, lote 2, en PostgreSQL local de HALCON.
- Se compiló y arrancó la versión actualizada en el puerto 3300; `/login` respondió HTTP 200 y contiene el token CSRF con el almacén Redis real.
- La instancia PostgreSQL temporal de pruebas fue detenida. El GPS físico y la ejecución con pantalla bloqueada no se comprobaron; la transmisión del navegador requiere una página abierta y permiso de ubicación.
