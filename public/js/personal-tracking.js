document.addEventListener('DOMContentLoaded', function () {
  'use strict';
  const start = document.getElementById('share-start');
  const stop = document.getElementById('share-stop');
  const status = document.getElementById('share-status');
  if (!start) return;
  let socket = null, watch = null, retry = null, heartbeat = null, enabled = false, last = null;
  function send() {
    if (socket && socket.readyState === WebSocket.OPEN && last && Date.now() - last.at < 45000) {
      socket.send(JSON.stringify({latitude: last.latitude, longitude: last.longitude}));
      status.textContent = 'Transmitiendo ubicación. Mantén esta página abierta.';
    }
  }
  function connect() {
    if (!enabled) return;
    socket = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/location');
    socket.onopen = send;
    socket.onclose = function (event) {
      socket = null;
      if (event.code === 4009) { halt('Otra pestaña o dispositivo está transmitiendo tu halcón.'); return; }
      if (event.code === 1008) { halt('Sesión finalizada o acceso revocado. Vuelve a iniciar sesión.'); return; }
      if (enabled) { status.textContent = 'Reconectando transmisión…'; retry = setTimeout(connect, 3000); }
    };
    socket.onerror = function () { if (socket) socket.close(); };
  }
  function halt(message) {
    enabled = false;
    clearTimeout(retry); clearInterval(heartbeat);
    if (watch !== null) navigator.geolocation.clearWatch(watch);
    watch = null; last = null;
    if (socket) { socket.onclose = null; socket.close(); socket = null; }
    start.disabled = false; stop.disabled = true;
    status.textContent = message || 'Transmisión detenida. El destinatario conserva acceso a la última ubicación hasta que retires su acceso.';
  }
  start.addEventListener('click', function () {
    if (!navigator.geolocation || !window.isSecureContext) { status.textContent = 'GPS requiere HTTPS (o localhost) y un navegador compatible.'; return; }
    enabled = true; start.disabled = true; stop.disabled = false;
    status.textContent = 'Esperando permiso y una posición GPS…';
    connect();
    watch = navigator.geolocation.watchPosition(function (p) {
      last = {latitude: p.coords.latitude, longitude: p.coords.longitude, at: p.timestamp}; send();
    }, function (err) {
      if (err.code === 1) halt('Permiso GPS denegado. Habilítalo en el navegador para transmitir.');
      else status.textContent = 'GPS no disponible. Esperando una posición nueva…';
    }, {enableHighAccuracy: true, maximumAge: 10000, timeout: 20000});
    heartbeat = setInterval(send, 15000);
  });
  stop.addEventListener('click', function () { halt(); });
  window.addEventListener('pagehide', function () { halt(); });
});
