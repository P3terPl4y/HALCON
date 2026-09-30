# Verificación de seguridad y experiencia de usuario

## Cambios

El backend sigue en Go, Fiber v3 y Goravel. Se incorporaron endpoints de sesión y seguimiento para el cliente React con las mismas cookies, controles de acceso y protección CSRF que las vistas del servidor. Los inicios de sesión tienen límite de intentos. La API autenticada responde 401 y evita cachear datos de sesión y ubicación. La lista de usuarios requiere moderador o administrador.

La revisión previa al push corrigió dos fallos de funcionamiento: Redis y el puerto ahora respetan `.env`, y la eliminación desde el formulario del moderador usa una ruta POST real con CSRF y verificación de propiedad. Se conserva la ruta DELETE para clientes existentes. React está incluido en `react/`, sin dependencias instaladas ni compilados.

El registro público fuerza el rol usuario, valida correo y limita la contraseña al rango admitido por bcrypt. Los fallos de creación de cuenta no exponen errores SQL. La selección por correo evita tener que conocer IDs y rechaza cuentas inactivas, destinatarios inexistentes o el propio usuario. El servidor determina siempre el propietario por la sesión.

Se eliminó la creación automática de un administrador con contraseña fija. Para crear uno de forma explícita, configura `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PASSWORD` (mínimo 12 caracteres) y opcionalmente `BOOTSTRAP_ADMIN_NAME`. Esto no cambia contraseñas de cuentas existentes: si se usó el administrador histórico con su contraseña publicada en el código anterior, su contraseña todavía debe rotarse.

El perfil ahora separa destinatario, transmisión y mapa. Guardar y retirar acceso muestran resultados sin recargar; los errores conservan el correo escrito. Los controles tienen etiquetas, foco visible, estado de conexión y botones de al menos 2.75rem. Se corrigieron el desbordamiento de la barra superior, controles anidados y marcadores sin nombre accesible. El CSS nuevo emplea rem, porcentajes, clamp, ch y dvh, conservando los tokens violeta, glaciar y colores semánticos existentes. Respeta la preferencia de movimiento reducido.

La organización y proximidad agrupan tareas relacionadas; la jerarquía reduce decisiones simultáneas; los controles amplios facilitan su selección y la respuesta inmediata muestra qué ocurrió. Detener GPS y retirar acceso tienen efectos distintos, explicados en la interfaz.

## Pruebas

- Go: compilación, `go test ./...`, `go vet ./...`, y pruebas de integración con PostgreSQL real y `-race`.
- Autorización: usuario común sin acceso al directorio; dashboard de otro usuario rechazado; moderador sin permiso para borrar o reasignar dispositivos ajenos; IDs de propietario y moderador falsificados ignorados o rechazados.
- Sesiones: CSRF real, login y registro, registro sin escalada de rol, logout y revocación de WebSocket; datos de API sin contraseña ni token de dispositivo; respuestas no cacheables; límite de intentos de login.
- Seguimiento: creación concurrente de un único halcón personal, migración/reversión/recuperación, precisión GPS, emisión y recepción, cambio y retiro de destinatario, reemplazo de emisor y comprobación de origen.
- Chrome y Playwright: las dos interfaces a 320, 390, 768 y 1440 px, sin desbordamiento horizontal; formularios sin recarga, teclado, mensajes de error, texto con intento XSS, conexión, GPS simulado y cierre de sesión en React.
- Moderador en Chrome: ve su dispositivo en el mapa y puede crear, asignar y eliminar un dispositivo mediante los formularios reales. Las tres pruebas de navegador finalizaron correctamente antes de publicar.
- Axe: páginas de seguimiento completas con reglas WCAG 2 A/AA, 2.1 AA y 2.2 AA, sin infracciones detectadas en el escenario probado. No sustituye pruebas manuales con tecnologías de asistencia ni certifica las demás páginas administrativas.
- React: comprobación estricta de TypeScript y compilación Vite aprobadas; `npm audit` informó cero vulnerabilidades conocidas el 1 de octubre de 2026.

Los tests de integración son optativos y trabajan en esquemas temporales. Ejecuta cada fixture de PostgreSQL por separado, ya que el descubrimiento de tablas del driver puede ver tablas de otro esquema temporal concurrente. El fixture de navegador verifica el esquema antes de escribir y dispone de cierre acotado del proceso hijo.

Consulta los comandos en `PLAN_ACTUALIZACION.md` y en `react/README.md`. Los tests de navegador están en el proyecto React; las pruebas de seguridad del backend están en `tracking_integration_test.go` y `app/websockets/handler_test.go`.

## Alcance operativo

La comprobación usa Chrome y GPS simulado, no un teléfono físico ni un examen de penetración independiente. HTTPS y permiso de ubicación son necesarios fuera de localhost. La página debe permanecer abierta. Se conserva la última posición al detener GPS; retirar acceso impide nuevas consultas del destinatario. La sincronización del hub entre varios procesos sigue pendiente, por lo que se mantiene la recomendación de un proceso de aplicación.

Documentación utilizada: [Fiber v3 Limiter](https://docs.gofiber.io/next/middleware/limiter/), [React useEffect](https://react.dev/reference/react/useEffect), [Vite](https://vite.dev/guide/), además de las fuentes de Goravel y WebSocket indicadas en el plan.
