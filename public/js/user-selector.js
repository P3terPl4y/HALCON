document.addEventListener('DOMContentLoaded', function () {
  'use strict';
  document.querySelectorAll('select[name="user_id"]').forEach(function (select, index) {
    const label = document.createElement('label'), input = document.createElement('input'), status = document.createElement('p');
    const id = 'user-search-' + index;
    label.htmlFor = id; label.textContent = 'Buscar usuario por nombre o correo'; label.className = 'form-label';
    input.id = id; input.type = 'search'; input.className = 'form-control'; input.maxLength = 254; input.autocomplete = 'off';
    status.id = id + '-status'; status.className = 'tracking-help'; status.setAttribute('role', 'status'); input.setAttribute('aria-describedby', status.id);
    select.before(label, input, status);
    let timer, controller, version = 0;
    input.addEventListener('input', function () {
      clearTimeout(timer); if (controller) controller.abort(); const current = ++version;
      timer = setTimeout(async function () {
        controller = new AbortController(); status.textContent = 'Buscando usuarios…';
        try {
          const response = await fetch('/users?limit=100&search=' + encodeURIComponent(input.value), {credentials:'same-origin',signal:controller.signal,cache:'no-store'});
          if (response.redirected) throw new Error('Tu sesión terminó. Inicia sesión de nuevo.');
          if (!response.ok) throw new Error(response.status === 401 ? 'Tu sesión terminó. Inicia sesión de nuevo.' : 'No se pudo buscar. Inténtalo de nuevo.');
          const data = await response.json(); if (current !== version) return;
          const value = select.value, selected = select.selectedOptions[0];
          select.replaceChildren(new Option('Selecciona un usuario…', ''));
          data.users.forEach(function (u) { select.add(new Option(u.name + ' (' + u.email + ')', String(u.id))); });
          if (value && !data.users.some(function (u) { return String(u.id) === value; }) && selected) select.add(new Option(selected.textContent, value));
          select.value = value;
          status.textContent = data.total > data.users.length ? 'Se muestran ' + data.users.length + ' de ' + data.total + ' usuarios. Escribe un correo más preciso.' : data.total + ' usuarios encontrados.';
        } catch (error) { if (error.name !== 'AbortError' && current === version) status.textContent = error.message; }
      }, 250);
    });
  });
});
