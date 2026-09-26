// panel.js — dev-панель эмулятора: уровни, время (ползунок/пауза), коды (вкл/выкл),
// штрафные подсказки, автопереход. Рисуется в Shadow DOM поверх страницы (z-index 10000,
// выше тумана fog.js = 9998) и не зависит от CSS игры. Данные берёт из /api/level/{n};
// сама страница остаётся байт в байт как у движка.
(function () {
  if (window.__emuPanel) return;
  window.__emuPanel = true;
  var lvlInput = document.querySelector('input[name="LevelNumber"]');
  var LEVEL = lvlInput ? +lvlInput.value : 0;
  if (!LEVEL) return;

  var host = document.getElementById('emu-panel-host');
  if (!host) { host = document.createElement('div'); host.id = 'emu-panel-host'; document.body.appendChild(host); }
  var root = host.attachShadow({ mode: 'open' });
  var P = null;

  var CSS = [
    ':host{all:initial}',
    '*{box-sizing:border-box}',
    '.drawer{position:fixed;top:0;right:0;height:100vh;width:340px;max-width:92vw;z-index:10000;background:#15161a;color:#e6e6e6;font:13px/1.4 Segoe UI,Roboto,Arial,sans-serif;border-left:1px solid #333;box-shadow:-8px 0 24px rgba(0,0,0,.5);display:flex;flex-direction:column;transform:translateX(0);transition:transform .2s}',
    '.drawer.collapsed{transform:translateX(100%)}',
    '.tab{position:fixed;top:12px;right:0;z-index:10001;background:#2b7a3e;color:#fff;border:0;border-radius:6px 0 0 6px;padding:8px 10px;font:600 12px Segoe UI,Arial,sans-serif;cursor:pointer;box-shadow:0 2px 8px rgba(0,0,0,.4)}',
    '.tab.open{right:340px}',
    '@media (max-width:400px){.tab.open{right:92vw}}',
    'header{padding:10px 12px;border-bottom:1px solid #333;display:flex;align-items:center;gap:8px}',
    'header b{font-size:14px}',
    'header .sp{flex:1}',
    '.body{overflow:auto;flex:1;padding:8px 12px 20px}',
    'section{margin:0 0 14px}',
    'h4{margin:0 0 6px;font:600 11px Segoe UI,Arial,sans-serif;letter-spacing:.08em;text-transform:uppercase;color:#8a8f99}',
    'button{background:#2a2c33;color:#e6e6e6;border:1px solid #444;border-radius:5px;padding:4px 9px;font:12px Segoe UI,Arial,sans-serif;cursor:pointer}',
    'button:hover{background:#363945}',
    'button.primary{background:#2b7a3e;border-color:#2b7a3e}',
    'button.danger{background:#7a2b2b;border-color:#7a2b2b}',
    'button.small{padding:2px 6px;font-size:11px}',
    'select,input[type=range]{width:100%}',
    'select{background:#2a2c33;color:#e6e6e6;border:1px solid #444;border-radius:5px;padding:4px}',
    '.clock{font:700 22px Consolas,monospace;color:#ffd166}',
    '.clock.paused{color:#ff7b7b}',
    '.row{display:flex;gap:6px;align-items:center;flex-wrap:wrap;margin:4px 0}',
    '.code{display:flex;gap:8px;align-items:flex-start;padding:5px 6px;border-radius:5px;border:1px solid #2a2c33;margin:3px 0;background:#1b1c21}',
    '.code.on{border-color:#2b7a3e;background:#172a1c}',
    '.code input{margin-top:3px}',
    '.code .t{font-size:10px;padding:1px 5px;border-radius:3px;background:#333;color:#ccc;white-space:nowrap}',
    '.code .t.sec{background:#2d4d7a}.code .t.bon{background:#2b7a3e}.code .t.pen{background:#7a2b2b}.code .t.mix{background:#6b4d1f}',
    '.code .n{color:#fff}',
    '.code .a{color:#9ad;font-family:Consolas,monospace}',
    '.code .f{color:#8a8f99;font-size:11px}',
    '.muted{color:#8a8f99;font-size:11px}',
    '.warn{color:#ffb4a2;font-size:11px;white-space:pre-wrap}',
    '.toggle{display:flex;align-items:center;gap:6px;cursor:pointer}',
    '.badge{display:inline-block;padding:1px 6px;border-radius:10px;background:#333;font-size:10px}',
    '.badge.ok{background:#2b7a3e}.badge.to{background:#7a2b2b}',
    'kbd{background:#333;border-radius:3px;padding:0 4px;font-size:10px}'
  ].join('\n');

  function fmt(s) {
    s = Math.max(0, s | 0);
    var h = (s / 3600) | 0, m = ((s % 3600) / 60) | 0, x = s % 60;
    return (h ? h + ':' + String(m).padStart(2, '0') : String(m).padStart(2, '0')) + ':' + String(x).padStart(2, '0');
  }
  function esc(s) { return String(s == null ? '' : s).replace(/[&<>"]/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]; }); }
  function nav(url) { location.href = url || (P && P.playPath) || location.pathname; }
  function post(body) {
    return fetch('/api/state/' + LEVEL, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
      .then(function (r) { return r.json(); })
      .then(function (j) {
        if (j.error) { warn(j.error); return; }
        nav(j.redirect);
      })
      .catch(function (e) { warn(String(e)); });
  }
  function warn(msg) { var w = root.getElementById('warn'); if (w) { w.textContent = msg; } }

  var collapsed = false;
  try { collapsed = localStorage.getItem('emu-panel-collapsed') === '1'; } catch (e) {}

  function typeBadge(c) {
    var cls = 'sec';
    if (c.type === 'бонус') cls = 'bon';
    else if (c.type === 'штраф') cls = 'pen';
    else if (c.type === 'секторбонус' || c.type === 'секторштраф') cls = 'mix';
    return '<span class="t ' + cls + '">' + esc(c.type) + '</span>';
  }

  var elapsed = 0;
  function render() {
    var levelOpts = P.levels.map(function (l) {
      var mark = l.passed ? ' ✓' : (l.started ? ' ·' : '');
      return '<option value="' + l.number + '"' + (l.current ? ' selected' : '') + '>' + l.number + (l.name ? ' — ' + esc(l.name) : '') + mark + '</option>';
    }).join('');

    var codes = P.codes.map(function (c) {
      var name = c.kind === 'sector' ? (c.sectorName || '(сектор)') : (c.bonusName || '(бонус)');
      if (c.kind === 'sector' && c.bonusName && c.bonusName !== c.sectorName) name += ' / ' + c.bonusName;
      var extra = [];
      if (c.time) extra.push((c.negative ? '+' : '−') + fmt(c.time));
      if (c.foreign) extra.push('с уровня ' + c.ownerLevel);
      return '<label class="code' + (c.entered ? ' on' : '') + '">' +
        '<input type="checkbox" data-key="' + esc(c.key) + '"' + (c.entered ? ' checked' : '') + '>' +
        '<div><div>' + typeBadge(c) + ' <span class="n">' + esc(name) + '</span></div>' +
        '<div class="a">' + esc(c.answers.join(' · ')) + '</div>' +
        (extra.length ? '<div class="f">' + esc(extra.join(' · ')) + '</div>' : '') +
        '</div></label>';
    }).join('') || '<div class="muted">На уровне нет кодов.</div>';

    var pens = (P.penalties || []).map(function (h) {
      var st = h.state === 2 ? '<span class="badge ok">открыта</span>' : (h.remain > 0 ? '<span class="badge">через ' + fmt(h.remain) + '</span>' : '<span class="badge">доступна</span>');
      return '<div class="row"><span>Штрафная ' + (h.index + 1) + ' (' + fmt(h.penalty) + ')</span> ' + st + ' <span class="sp"></span>' +
        (h.state === 2 ? '<button class="small" data-hint-close="' + h.index + '">закрыть</button>' :
          '<button class="small" data-hint-open="' + h.index + '">открыть</button>') +
        '</div>';
    }).join('');

    var hints = (P.hints || []).map(function (h) {
      return '<span class="badge' + (h.shown ? ' ok' : '') + '">П' + h.number + ' @ ' + fmt(h.time) + '</span> ';
    }).join('');

    var status = P.passed ? '<span class="badge ' + (P.passedBy === 'timeout' ? 'to' : 'ok') + '">пройден' + (P.passedBy === 'timeout' ? ' по времени' : '') + '</span>' : '<span class="badge">' + P.closed + '/' + P.required + ' секторов</span>';

    root.innerHTML =
      '<style>' + CSS + '</style>' +
      '<button class="tab' + (collapsed ? '' : ' open') + '" id="tab">' + (collapsed ? '◀ EMU' : '▶') + '</button>' +
      '<div class="drawer' + (collapsed ? ' collapsed' : '') + '" id="drawer">' +
      '<header><b>Эмулятор</b> <span class="muted">уровень ' + P.level + '</span><span class="sp"></span>' + status + '</header>' +
      '<div class="body">' +
      '<div class="warn" id="warn">' + (P.missing && P.missing.length ? 'Нет файлов ассетов: ' + esc(P.missing.join(', ')) : '') + '</div>' +

      '<section><h4>Уровень</h4><select id="level">' + levelOpts + '</select>' +
      '<div class="row"><button id="prev">◀ пред.</button><button id="next">след. ▶</button><span class="sp"></span><button class="danger small" id="reset">сброс уровня</button><button class="danger small" id="resetAll">сброс всего</button></div></section>' +

      '<section><h4>Время уровня</h4>' +
      '<div class="row"><span class="clock' + (P.paused ? ' paused' : '') + '" id="clock">' + fmt(P.elapsed) + '</span><span class="sp"></span>' +
      '<button id="pause" class="' + (P.paused ? 'primary' : '') + '">' + (P.paused ? '▶ пуск' : '⏸ пауза') + '</button></div>' +
      '<input type="range" id="slider" min="0" max="' + P.sliderMax + '" step="1" value="' + P.elapsed + '">' +
      '<div class="row"><button class="small" data-plus="-60">−1м</button><button class="small" data-plus="60">+1м</button><button class="small" data-plus="300">+5м</button><button class="small" data-plus="600">+10м</button><button class="small" id="zero">= 0</button></div>' +
      (P.timeout ? '<div class="muted">Автопереход через ' + fmt(P.timeout) + '</div>' : '') +
      (hints ? '<div class="muted" style="margin-top:4px">Подсказки: ' + hints + '</div>' : '') +
      '</section>' +

      (pens ? '<section><h4>Штрафные подсказки</h4>' + pens + '</section>' : '') +

      '<section><h4>Коды</h4>' + codes + '</section>' +

      '<section><h4>Настройки</h4>' +
      '<label class="toggle"><input type="checkbox" id="auto"' + (P.autoAdvance ? ' checked' : '') + '> переход на следующий уровень, когда задание выполнено</label>' +
      '<div class="muted" style="margin-top:6px">Свернуть/развернуть: <kbd>Ctrl</kbd>+<kbd>`</kbd>. Конфиги и ассеты перечитываются при каждом обновлении; страница обновится сама при изменении файлов.</div>' +
      '</section>' +
      '</div></div>';

    root.getElementById('tab').onclick = toggle;
    root.getElementById('level').onchange = function (e) { post({ action: 'goto', level: +e.target.value }); };
    root.getElementById('prev').onclick = function () { var i = idx(); if (i > 0) post({ action: 'goto', level: P.levels[i - 1].number }); };
    root.getElementById('next').onclick = function () { var i = idx(); if (i < P.levels.length - 1) post({ action: 'goto', level: P.levels[i + 1].number }); };
    root.getElementById('reset').onclick = function () { post({ action: 'reset' }); };
    root.getElementById('resetAll').onclick = function () { post({ action: 'resetAll' }); };
    root.getElementById('pause').onclick = function () { post({ action: 'pause', on: !P.paused }); };
    root.getElementById('zero').onclick = function () { post({ action: 'setElapsed', seconds: 0 }); };
    root.getElementById('auto').onchange = function (e) { post({ action: 'autoAdvance', on: e.target.checked }); };
    var slider = root.getElementById('slider'), clock = root.getElementById('clock');
    var dragging = false;
    slider.oninput = function () { dragging = true; clock.textContent = fmt(+slider.value); };
    slider.onchange = function () { dragging = false; post({ action: 'setElapsed', seconds: +slider.value }); };
    root.querySelectorAll('[data-plus]').forEach(function (b) {
      b.onclick = function () { post({ action: 'setElapsed', seconds: Math.max(0, elapsed + (+b.getAttribute('data-plus'))) }); };
    });
    root.querySelectorAll('input[data-key]').forEach(function (cb) {
      cb.onchange = function () { post({ action: 'toggleCode', key: cb.getAttribute('data-key'), on: cb.checked }); };
    });
    root.querySelectorAll('[data-hint-open]').forEach(function (b) { b.onclick = function () { post({ action: 'openHint', index: +b.getAttribute('data-hint-open') }); }; });
    root.querySelectorAll('[data-hint-close]').forEach(function (b) { b.onclick = function () { post({ action: 'closeHint', index: +b.getAttribute('data-hint-close') }); }; });

    setInterval(function () {
      if (P.paused || dragging) return;
      elapsed++;
      clock.textContent = fmt(elapsed);
      if (elapsed <= +slider.max) slider.value = elapsed;
    }, 1000);
  }

  function idx() { for (var i = 0; i < P.levels.length; i++) if (P.levels[i].current) return i; return 0; }
  function toggle() {
    collapsed = !collapsed;
    try { localStorage.setItem('emu-panel-collapsed', collapsed ? '1' : '0'); } catch (e) {}
    root.getElementById('drawer').classList.toggle('collapsed', collapsed);
    var t = root.getElementById('tab');
    t.classList.toggle('open', !collapsed);
    t.textContent = collapsed ? '◀ EMU' : '▶';
  }

  fetch('/api/level/' + LEVEL).then(function (r) { return r.json(); }).then(function (j) {
    P = j.panel;
    elapsed = P.elapsed;
    render();
  }).catch(function (e) { root.innerHTML = '<div style="position:fixed;right:0;top:0;background:#7a2b2b;color:#fff;padding:6px 10px;z-index:10000;font:12px sans-serif">emu panel: ' + esc(String(e)) + '</div>'; });

  document.addEventListener('keydown', function (e) {
    if (e.ctrlKey && (e.code === 'Backquote' || e.key === '`' || e.key === 'ё')) { e.preventDefault(); toggle(); }
  });

  // live reload: при изменении конфигов/ассетов — GET той же страницы
  var mtime = null;
  setInterval(function () {
    fetch('/api/mtime').then(function (r) { return r.json(); }).then(function (j) {
      if (mtime === null) mtime = j.mtime;
      else if (j.mtime !== mtime) nav();
    }).catch(function () {});
  }, 1000);
})();
