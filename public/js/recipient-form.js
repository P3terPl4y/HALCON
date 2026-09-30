document.addEventListener('DOMContentLoaded', function () {
  'use strict';
  const form = document.getElementById('recipient-form');
  if (!form || !window.fetch) return;
  const feedback = document.getElementById('recipient-feedback');
  const current = document.getElementById('recipient-current');
  const remove = document.getElementById('recipient-remove');
  const input = document.getElementById('recipient-email');
  const save = document.getElementById('recipient-save');
  form.addEventListener('submit', async function (event) {
    event.preventDefault();
    if (form.getAttribute('aria-busy') === 'true') return;
    const removing = event.submitter === remove;
    const payload = new URLSearchParams(new FormData(form));
    if (removing) payload.set('recipient_id', '0');
    form.setAttribute('aria-busy', 'true'); save.disabled = true; remove.disabled = true;
    feedback.textContent = removing ? 'Retirando acceso…' : 'Guardando destinatario…';
    let hasRecipient = !remove.hasAttribute('data-empty');
    try {
      const response = await fetch('/api/tracking/recipient', {method: 'POST', body: payload, credentials: 'same-origin', headers: {Accept: 'application/json'}});
      if (response.status === 401) { feedback.textContent = 'Tu sesión terminó. Inicia sesión para continuar.'; return; }
      if (response.status === 403) { feedback.textContent = 'El formulario caducó. Recarga la página e inténtalo de nuevo.'; return; }
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'No se pudo guardar. Inténtalo de nuevo.');
      hasRecipient = Boolean(data.recipient);
      current.textContent = data.recipient ? 'Compartiendo con ' + data.recipient.name : 'Sin destinatario. Tu ubicación personal aún no se comparte.';
      input.value = data.recipient ? data.recipient.email : '';
      form.querySelector('[name="_csrf"]').value = data.csrf_token;
      feedback.textContent = removing ? 'Acceso retirado. Esa persona ya no recibirá tu ubicación.' : 'Destinatario guardado. Puedes activar la ubicación.';
      input.removeAttribute('aria-invalid');
    } catch (error) {
      feedback.textContent = error.message === 'Failed to fetch' ? 'No hay conexión. Conservamos el correo para que puedas reintentar.' : error.message;
      input.setAttribute('aria-invalid', 'true');
    } finally {
      form.setAttribute('aria-busy', 'false'); save.disabled = false; remove.disabled = !hasRecipient;
      remove.toggleAttribute('data-empty', !hasRecipient);
    }
  });
  remove.toggleAttribute('data-empty', remove.disabled);
});
