(function(){
  'use strict';

  try {

    /* ============================================================
       TEMA (mismo localStorage key que landing y dashboard)
       ============================================================ */
    var root = document.documentElement;
    var themeBtn = document.getElementById('authThemeToggle');

    function apply(theme){
      root.setAttribute('data-theme', theme);
      if(themeBtn) themeBtn.setAttribute('aria-pressed', theme === 'dark' ? 'true' : 'false');
    }

    try{
      var saved = localStorage.getItem('halcon-theme');
      var prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
      apply(saved || (prefersDark ? 'dark' : 'light'));
    }catch(e){}

    if(themeBtn){
      themeBtn.addEventListener('click', function(){
        var cur = root.getAttribute('data-theme') === 'dark' ? 'dark' : 'light';
        var next = cur === 'dark' ? 'light' : 'dark';
        apply(next);
        try{ localStorage.setItem('halcon-theme', next); }catch(e){}
      });
    }

    /* ============================================================
       PASSWORD TOGGLE
       Soporta múltiples toggle por página (login y register).
       ============================================================ */
    document.querySelectorAll('[data-pw-toggle]').forEach(function(btn){
      var targetId = btn.getAttribute('data-pw-toggle');
      var input = document.getElementById(targetId);
      if(!input) return;

      btn.addEventListener('click', function(){
        var showing = input.type === 'text';
        input.type = showing ? 'password' : 'text';
        btn.setAttribute('aria-pressed', showing ? 'false' : 'true');
        btn.setAttribute('aria-label', showing ? 'Mostrar contraseña' : 'Ocultar contraseña');
      });
    });

     /* ============================================================
       PASSWORD STRENGTH (solo register)
       Score 0-4. Muestra "Muy débil" desde el primer carácter.
       ============================================================ */
    var pwInput = document.getElementById('password');
    var strengthEl = document.getElementById('pwStrength');
    var strengthLabel = document.getElementById('pwStrengthLabel');

    if(pwInput && strengthEl){
      var labels = ['Muy débil', 'Débil', 'Aceptable', 'Fuerte', 'Excelente'];

      function scorePassword(pw){
        if(!pw) return 0;
        var score = 0;
        if(pw.length >= 8)  score++;
        if(pw.length >= 12) score++;
        if(/[a-z]/.test(pw) && /[A-Z]/.test(pw)) score++;
        if(/\d/.test(pw))                        score++;
        if(/[^A-Za-z0-9]/.test(pw))              score++;
        return Math.min(score, 4);
      }

      function updateStrength(){
        var value = pwInput.value;
        var score = scorePassword(value);

        strengthEl.setAttribute('data-score', String(score));

        if(strengthLabel){
          strengthLabel.textContent = value ? labels[score] : '';
        }
      }

      pwInput.addEventListener('input', updateStrength);
      updateStrength(); // por si el navegador autocompletó
    }
    /* ============================================================
       CONFIRM PASSWORD MATCH
       Muestra un pequeño texto "Las contraseñas no coinciden"
       si el usuario escribe algo distinto en confirm.
       ============================================================ */
    var pwConfirm = document.getElementById('password_confirm');
    if(pwInput && pwConfirm){
      var matchMsg = document.getElementById('pwMatchMsg');
      function checkMatch(){
        if(!pwConfirm.value){
          if(matchMsg) matchMsg.textContent = '';
          return;
        }
        var ok = pwInput.value === pwConfirm.value;
        if(matchMsg){
          matchMsg.textContent = ok ? '✓ Las contraseñas coinciden' : 'Las contraseñas no coinciden';
          matchMsg.style.color = ok ? 'var(--success)' : 'var(--danger)';
        }
      }
      pwInput.addEventListener('input', checkMatch);
      pwConfirm.addEventListener('input', checkMatch);
    }

    /* ============================================================
       PREVENIR DOBLE SUBMIT
       ============================================================ */
    document.querySelectorAll('form[data-guard-submit]').forEach(function(form){
      form.addEventListener('submit', function(){
        var btn = form.querySelector('button[type="submit"]');
        if(btn){
          btn.disabled = true;
          var original = btn.innerHTML;
          btn.innerHTML = '<span>Procesando…</span>';
          // Restaurar por si el server rechaza y el navegador vuelve atrás
          setTimeout(function(){
            if(btn.disabled){
              btn.disabled = false;
              btn.innerHTML = original;
            }
          }, 8000);
        }
      });
    });

  } catch(err){
    try { console.error('[HALCON auth]', err); } catch(_){}
  }
})();
