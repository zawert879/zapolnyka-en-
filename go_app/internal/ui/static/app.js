// app.js — веб-интерфейс zapolnyaka: вкладки Команды / Коды / Редактор / Превью / Эмулятор.
// Одна страница без сборки: состояние в S, данные через /api/ui/*, эмулятор — в iframe.
(function () {
  'use strict';
  const $ = (sel, root) => (root || document).querySelector(sel);
  const $$ = (sel, root) => Array.from((root || document).querySelectorAll(sel));
  const esc = (s) => String(s == null ? '' : s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const fmt = (s) => { s = Math.max(0, s | 0); const h = (s / 3600) | 0, m = ((s % 3600) / 60) | 0, x = s % 60; return (h ? h + ':' + String(m).padStart(2, '0') : String(m).padStart(2, '0')) + ':' + String(x).padStart(2, '0'); };

  const S = {
    state: null, level: 0, data: null, tab: 'commands',
    selected: new Set(), codes: [], codesDirty: false, sel: -1,
    file: 'body', raw: {}, rawDirty: {}, conf: null, confDirty: false,
    job: null, jobFrom: 0, jobTimer: null, previewVW: '1280',
  };

  async function api(method, url, body, rawBody) {
    const opt = { method, headers: {} };
    if (body !== undefined) {
      if (rawBody) { opt.body = body; opt.headers['Content-Type'] = 'text/plain; charset=utf-8'; }
      else { opt.body = JSON.stringify(body); opt.headers['Content-Type'] = 'application/json'; }
    }
    const r = await fetch(url, opt);
    const ct = r.headers.get('content-type') || '';
    const data = ct.includes('json') ? await r.json() : await r.text();
    if (!r.ok) throw new Error((data && data.error) || (typeof data === 'string' ? data : r.statusText));
    return data;
  }

  let toastTimer;
  function toast(msg, kind) {
    const t = $('#toast');
    t.textContent = msg; t.className = 'toast ' + (kind || ''); t.hidden = false;
    clearTimeout(toastTimer); toastTimer = setTimeout(() => { t.hidden = true; }, kind === 'err' ? 6000 : 2500);
  }

  // ---------------------------------------------------------------- состояние и оболочка
  async function loadState() {
    S.state = await api('GET', '/api/ui/state');
    const st = S.state;
    const sel = $('#gameSelect');
    sel.innerHTML = st.games.map((g) => `<option value="${esc(g.path)}"${g.current ? ' selected' : ''}>${esc(g.title || g.path)} · ${esc(g.domain)} · #${g.gameId} · ${esc(g.path)}</option>`).join('');
    if (!st.games.some((g) => g.current) && st.game) sel.insertAdjacentHTML('afterbegin', `<option value="${esc(st.game.path)}" selected>${esc(st.game.title || st.game.path)} · ${esc(st.game.domain)} · #${st.game.gameId}</option>`);
    $('#authLogin').textContent = st.login || 'нет логина';
    $('#authDot').className = 'dot' + (st.login ? ' on' : '');
    $('#stAddr').textContent = 'сервер ' + st.addr;
    $('#stVersion').textContent = 'zapolnyaka ' + st.version;
    $('#emuUrl').textContent = st.addr + st.playPath;
    $('#emuNewWindow').href = st.playPath;
    if (st.error) toast(st.error, 'err');
    if (!st.levels.some((l) => l.number === S.level)) S.level = st.levels.length ? st.levels[0].number : 0;
    renderLevels();
    renderAssets();
    if (st.job) $('#stJob').textContent = 'последнее: ' + st.job.title + (st.job.done ? (st.job.error ? ' ✗ ' : ' ✔ ') + (st.job.finished || '') : ' …');
  }

  function renderLevels() {
    const box = $('#levels');
    box.innerHTML = S.state.levels.map((l) => `
      <div class="level${l.number === S.level ? ' active' : ''}" data-level="${l.number}" title="${esc(l.error || l.dir)}">
        <input type="checkbox" data-sel="${l.number}"${S.selected.has(l.number) ? ' checked' : ''} aria-label="Выбрать уровень ${l.number}" ${S.tab === 'commands' ? '' : 'hidden'}>
        <span class="num">${String(l.number).padStart(2, '0')}</span>
        <span class="name">${esc(l.name || l.dir)}</span>
        <span class="meta">${l.codes ? l.codes + ' код.' : ''}</span>
        <span class="st${l.error ? ' err' : ''}"></span>
      </div>`).join('') || '<div class="muted" style="padding:8px">В игре нет уровней — добавьте «+».</div>';
    $$('.level', box).forEach((el) => el.addEventListener('click', (e) => {
      if (e.target.matches('input[data-sel]')) { const n = +e.target.dataset.sel; if (e.target.checked) S.selected.add(n); else S.selected.delete(n); renderSelectedChip(); return; }
      selectLevel(+el.dataset.level);
    }));
    renderSelectedChip();
  }

  function renderSelectedChip() {
    const arr = Array.from(S.selected).sort((a, b) => a - b);
    $('#selectedChip').textContent = arr.length ? arr.map((n) => String(n).padStart(2, '0')).join(', ') : 'все';
  }

  function renderAssets() {
    $('#assets').innerHTML = S.state.assets.map((a) => `<div><span>${esc(a.name)}</span><span class="${a.uploaded ? 'up' : 'no'}">${a.uploaded ? 'залит' : 'не залит'}</span></div>`).join('') || '<div class="muted">папка ассетов пуста</div>';
    $('#assetButtons').innerHTML = S.state.assets.map((a) => `<button class="mono small" data-insert="{{${esc(a.name)}}}">{{${esc(a.name)}}}</button>`).join('') || '<span class="muted">нет ассетов</span>';
    $$('#assetButtons button').forEach((b) => b.addEventListener('click', () => insertAtCursor(b.dataset.insert)));
  }

  async function selectLevel(n) {
    if (S.codesDirty || Object.values(S.rawDirty).some(Boolean) || S.confDirty) {
      if (!confirm('Есть несохранённые изменения. Переключить уровень и потерять их?')) return;
    }
    S.level = n; S.codesDirty = false; S.rawDirty = {}; S.confDirty = false; S.sel = -1;
    renderLevels();
    await loadLevel();
    refreshTab();
  }

  async function loadLevel() {
    if (!S.level) { S.data = null; return; }
    try {
      S.data = await api('GET', '/api/ui/level/' + S.level);
      S.codes = JSON.parse(JSON.stringify(S.data.codes || []));
      S.raw = Object.assign({ body: '', conf: '', codes: '' }, S.data.raw || {});
      S.conf = JSON.parse(JSON.stringify(S.data.conf || {}));
      if (S.data.error) toast('Ошибка в файлах уровня: ' + S.data.error, 'err');
    } catch (e) { S.data = null; toast(String(e.message || e), 'err'); }
    updateDirty();
  }

  function updateDirty() {
    const d = [];
    if (S.codesDirty) d.push('codes.yml');
    if (S.confDirty) d.push('conf.yml');
    Object.keys(S.rawDirty).forEach((k) => { if (S.rawDirty[k]) d.push({ body: 'task.html', conf: 'conf.yml', codes: 'codes.yml' }[k]); });
    $('#stDirty').textContent = d.length ? 'не сохранено: ' + Array.from(new Set(d)).join(', ') : '';
    $('#stDirty').className = d.length ? 'dirty' : '';
    $$('.filetabs button[data-file]').forEach((b) => b.classList.toggle('dirty', !!S.rawDirty[b.dataset.file]));
  }

  // ---------------------------------------------------------------- вкладки
  function setTab(tab) {
    S.tab = tab;
    $$('#tabs a').forEach((a) => a.classList.toggle('active', a.dataset.tab === tab));
    $$('.tab').forEach((s) => s.classList.toggle('active', s.id === 'tab-' + tab));
    $$('#levels input[data-sel]').forEach((i) => { i.hidden = tab !== 'commands'; });
    refreshTab();
  }
  function refreshTab() {
    if (S.tab === 'codes') renderCodes();
    if (S.tab === 'editor') renderEditor();
    if (S.tab === 'preview') renderPreview();
    if (S.tab === 'emu') renderEmu();
  }
  window.addEventListener('hashchange', () => setTab((location.hash || '#commands').slice(1)));

  // ---------------------------------------------------------------- команды и задания
  async function run(cmd) {
    const body = { cmd };
    if (cmd === 'go' && $('#onlySelected').checked && S.selected.size) body.levels = Array.from(S.selected);
    if (cmd === 'snapshot') { body.name = $('#snapName').value.trim(); body.send = $('#snapSend').value.trim(); body.pid = +($('#snapPid').value || 0); body.pact = 1; }
    try {
      const r = await api('POST', '/api/ui/run', body);
      $('#log').textContent = '';
      S.job = r.id; S.jobFrom = 0;
      $('#jobTitle').textContent = cmd;
      $('#jobProgress').style.width = '0%';
      pollJob();
    } catch (e) { toast(String(e.message || e), 'err'); }
  }
  async function pollJob() {
    clearTimeout(S.jobTimer);
    if (!S.job) return;
    try {
      const r = await api('GET', `/api/ui/jobs/${S.job}?from=${S.jobFrom}`);
      const log = $('#log');
      r.lines.forEach((line) => {
        const cls = /^❌|ERROR|✗/.test(line) ? 'err' : /✔|✓|🎉/.test(line) ? 'ok' : /⚠|предупрежд/i.test(line) ? 'warn' : '';
        const span = document.createElement('span'); span.className = cls; span.textContent = line + '\n'; log.appendChild(span);
      });
      if (r.lines.length) log.scrollTop = log.scrollHeight;
      S.jobFrom = r.total;
      $('#jobTitle').textContent = r.job.title;
      const done = (log.textContent.match(/✔ уровень \d+ завершён/g) || []).length;
      const total = (S.selected.size && $('#onlySelected').checked) ? S.selected.size : (S.state ? S.state.levels.length : 0);
      if (r.job.cmd === 'go' && total) $('#jobProgress').style.width = Math.min(100, Math.round(done / total * 100)) + '%';
      if (r.job.done) {
        $('#jobProgress').style.width = '100%';
        $('#jobState').textContent = r.job.error ? 'ошибка' : 'готово';
        $('#stJob').textContent = 'последнее: ' + r.job.title + (r.job.error ? ' ✗ ' : ' ✔ ') + r.job.finished;
        toast(r.job.error ? 'Ошибка: ' + r.job.error : r.job.title + ' — готово', r.job.error ? 'err' : 'ok');
        S.job = null;
        await loadState();
        if (['go', 'assets'].includes(r.job.cmd)) refreshTab();
        return;
      }
      $('#jobState').textContent = 'выполняется…';
      S.jobTimer = setTimeout(pollJob, 700);
    } catch (e) { toast(String(e.message || e), 'err'); S.job = null; }
  }
  $$('[data-run]').forEach((b) => b.addEventListener('click', () => run(b.dataset.run)));
  $('#btnClearLog').addEventListener('click', () => { $('#log').textContent = ''; });
  $('#btnAuth').addEventListener('click', async () => {
    try {
      const r = await api('POST', '/api/ui/auth', { login: $('#authLoginInput').value, password: $('#authPassInput').value });
      $('#authPassInput').value = ''; toast('Сохранено: ' + r.login, 'ok'); await loadState();
    } catch (e) { toast(String(e.message || e), 'err'); }
  });
  $('#btnNewGame').addEventListener('click', async () => {
    try {
      const r = await api('POST', '/api/ui/game/new', { name: $('#ngName').value.trim(), domain: $('#ngDomain').value.trim(), gameId: +$('#ngGid').value });
      toast('Создано: ' + r.path, 'ok');
      await api('POST', '/api/ui/game', { path: r.path }); await loadState(); await loadLevel(); refreshTab();
    } catch (e) { toast(String(e.message || e), 'err'); }
  });
  $('#btnNewLevel').addEventListener('click', async () => {
    const next = S.state && S.state.levels.length ? Math.max(...S.state.levels.map((l) => l.number)) + 1 : 1;
    const num = prompt('Номер уровня', String(next)); if (!num) return;
    const dir = prompt('Папка уровня (внутри папки игры)', String(num).padStart(2, '0')); if (!dir) return;
    try { await api('POST', '/api/ui/level/new', { dir, number: +num }); toast('Уровень ' + num + ' создан', 'ok'); await loadState(); await selectLevel(+num); }
    catch (e) { toast(String(e.message || e), 'err'); }
  });
  $('#gameSelect').addEventListener('change', async (e) => {
    try { await api('POST', '/api/ui/game', { path: e.target.value }); S.level = 0; S.selected.clear(); await loadState(); await loadLevel(); refreshTab(); }
    catch (err) { toast(String(err.message || err), 'err'); }
  });

  // ---------------------------------------------------------------- коды
  const TYPES = ['сектор', 'бонус', 'штраф', 'секторбонус', 'секторштраф'];
  const hasSector = (t) => t === 'сектор' || t === 'секторбонус' || t === 'секторштраф';
  const hasBonus = (t) => t !== 'сектор';
  const isPenalty = (t) => t === 'штраф' || t === 'секторштраф';

  function codesWarnings() {
    const w = [];
    S.codes.forEach((c, i) => {
      const n = i + 1;
      if (!c.answers || !c.answers.length) w.push(`Запись ${n}: нет ответов`);
      if (hasBonus(c.type) && !(c.time > 0)) w.push(`Запись ${n}: для типа «${c.type}» нужно время (сек)`);
      if (hasSector(c.type) && !c.sectorName) w.push(`Запись ${n}: у сектора нет имени (sectorName)`);
      if (hasBonus(c.type) && !c.bonusName) w.push(`Запись ${n}: у бонуса нет имени (bonusName)`);
      const names = ((c.sectorName || '') + ' ' + (c.bonusName || '') + ' ' + (c.task || '')).toLowerCase();
      (c.answers || []).forEach((a) => { if (a && a.length > 2 && names.includes(a.toLowerCase())) w.push(`Запись ${n}: ответ «${a}» виден игроку до ввода (имя или задание)`); });
      if (c.levels && c.type === 'сектор') w.push(`Запись ${n}: levels допустимо только для бонусов`);
    });
    return w;
  }

  function renderCodes() {
    const d = S.data;
    $('#codesTitle').textContent = d ? `Коды · уровень ${d.number}${d.conf && d.conf.name ? ' «' + d.conf.name + '»' : ''}` : 'Коды';
    $('#codesPath').textContent = d ? `${d.files.codes || 'файл codes не задан'} · ${S.codes.length} записей` : '';
    const w = codesWarnings();
    $('#codesWarn').hidden = !w.length; $('#codesWarn').textContent = w.join('\n');
    const tb = $('#codesTable tbody');
    tb.innerHTML = S.codes.map((c, i) => `<tr class="${i === S.sel ? 'sel' : ''}" data-i="${i}">
      <td class="n">${i + 1}</td>
      <td><span class="t ${esc(c.type)}">${esc(c.type)}</span></td>
      <td>${esc(c.sectorName || '—')}</td>
      <td>${esc(c.bonusName || '—')}</td>
      <td class="ans">${esc((c.answers || []).join(' · '))}</td>
      <td class="mono ${c.type === 'сектор' ? 'muted' : isPenalty(c.type) ? 'plus' : 'minus'}">${c.type === 'сектор' ? '—' : (isPenalty(c.type) ? '+' : '−') + fmt(c.time || 0)}</td>
      <td class="mono muted">${esc(c.levels || '—')}</td>
      <td><div class="rowbtns"><button title="Вверх" data-mv="-1">▲</button><button title="Вниз" data-mv="1">▼</button><button title="Удалить" data-del="1" style="color:#e5484d">✕</button></div></td>
    </tr>`).join('') || '<tr><td colspan="8" class="muted" style="padding:16px">Кодов нет — добавьте кнопками ниже.</td></tr>';
    $$('tr[data-i]', tb).forEach((tr) => tr.addEventListener('click', (e) => {
      const i = +tr.dataset.i;
      if (e.target.dataset.mv) { const j = i + (+e.target.dataset.mv); if (j < 0 || j >= S.codes.length) return; [S.codes[i], S.codes[j]] = [S.codes[j], S.codes[i]]; S.sel = j; markCodes(); return; }
      if (e.target.dataset.del) { S.codes.splice(i, 1); S.sel = -1; markCodes(); return; }
      S.sel = i; renderCodes();
    }));
    renderInspector();
  }
  function markCodes() { S.codesDirty = true; updateDirty(); renderCodes(); }

  function renderInspector() {
    const box = $('#inspector');
    const c = S.codes[S.sel];
    if (!c) { box.innerHTML = '<div class="muted">Выберите запись в таблице или добавьте новую.</div>'; return; }
    box.innerHTML = `
      <div class="row top"><span class="label">Запись ${S.sel + 1}</span><select id="inType">${TYPES.map((t) => `<option${t === c.type ? ' selected' : ''}>${t}</option>`).join('')}</select></div>
      ${hasSector(c.type) ? `<label>Сектор (видно игроку сразу)<input id="inSector" value="${esc(c.sectorName || '')}"></label>` : ''}
      ${hasBonus(c.type) ? `<label>Бонус (видно игроку сразу)<input id="inBonus" value="${esc(c.bonusName || '')}"></label>` : ''}
      <label>Ответы (Enter — добавить)<div class="chips" id="inChips">${(c.answers || []).map((a, k) => `<span>${esc(a)}<b data-rm="${k}" title="Удалить">×</b></span>`).join('')}<input id="inAnswer" placeholder="ещё ответ"></div></label>
      ${hasBonus(c.type) ? `<div class="grid2"><label>Время, сек<input id="inTime" class="mono" type="number" min="0" value="${c.time || ''}"></label><label>Уровни (levels)<input id="inLevels" class="mono" placeholder="только этот" value="${esc(c.levels || '')}"></label></div>
      <label>Задание бонуса (task) — видно до ввода<textarea id="inTask">${esc(c.task || '')}</textarea></label>
      <label>Help — показывается только после ввода<textarea id="inHelp">${esc(c.help || '')}</textarea></label>` : ''}
      <div class="row" style="margin-top:auto"><button id="inEmu" class="small">Ввести в эмуляторе</button><button id="inEmuOff" class="small">Откатить в эмуляторе</button></div>`;
    const bind = (id, key, num) => { const el = $(id); if (!el) return; el.addEventListener('input', () => { const v = num ? (+el.value || 0) : el.value; c[key] = (v === '' ? undefined : v); S.codesDirty = true; updateDirty(); renderTableOnly(); }); };
    $('#inType').addEventListener('change', (e) => { c.type = e.target.value; if (c.type === 'сектор') delete c.time; markCodes(); });
    bind('#inSector', 'sectorName'); bind('#inBonus', 'bonusName'); bind('#inTime', 'time', true); bind('#inLevels', 'levels'); bind('#inTask', 'task'); bind('#inHelp', 'help');
    const ans = $('#inAnswer');
    ans.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); const v = ans.value.trim(); if (!v) return; c.answers = (c.answers || []).concat([v]); ans.value = ''; markCodes(); $('#inAnswer').focus(); }
      if (e.key === 'Backspace' && !ans.value && (c.answers || []).length) { c.answers.pop(); markCodes(); $('#inAnswer').focus(); }
    });
    $$('#inChips b').forEach((b) => b.addEventListener('click', () => { c.answers.splice(+b.dataset.rm, 1); markCodes(); }));
    $('#inEmu').addEventListener('click', () => emuToggle(S.sel, true));
    $('#inEmuOff').addEventListener('click', () => emuToggle(S.sel, false));
  }
  function renderTableOnly() {
    // обновить строку без потери фокуса в инспекторе
    const tr = $(`#codesTable tr[data-i="${S.sel}"]`); if (!tr) return;
    const c = S.codes[S.sel];
    tr.children[2].textContent = c.sectorName || '—'; tr.children[3].textContent = c.bonusName || '—';
    tr.children[4].textContent = (c.answers || []).join(' · ');
    tr.children[5].textContent = c.type === 'сектор' ? '—' : (isPenalty(c.type) ? '+' : '−') + fmt(c.time || 0);
    tr.children[6].textContent = c.levels || '—';
    const w = codesWarnings(); $('#codesWarn').hidden = !w.length; $('#codesWarn').textContent = w.join('\n');
  }
  $$('[data-add]').forEach((b) => b.addEventListener('click', () => {
    const t = b.dataset.add;
    const c = { type: t, answers: [] };
    if (hasSector(t)) c.sectorName = 'Сектор ' + (S.codes.filter((x) => hasSector(x.type)).length + 1);
    if (hasBonus(t)) { c.bonusName = (isPenalty(t) ? 'Штраф ' : 'Бонус ') + (S.codes.filter((x) => hasBonus(x.type)).length + 1); c.time = isPenalty(t) ? 300 : 60; }
    S.codes.push(c); S.sel = S.codes.length - 1; markCodes();
    setTimeout(() => { const el = $('#inAnswer'); if (el) el.focus(); }, 0);
  }));
  $('#btnCodesSave').addEventListener('click', async () => {
    try {
      const codes = S.codes.map((c) => { const o = Object.assign({}, c); if (o.type === 'сектор') delete o.time; if (o.time === '') delete o.time; return o; });
      await api('PUT', `/api/ui/level/${S.level}/codes`, codes);
      S.codesDirty = false; toast('codes.yml сохранён', 'ok'); await loadState(); await loadLevel(); renderCodes();
    } catch (e) { toast(String(e.message || e), 'err'); }
  });
  $('#btnCodesReload').addEventListener('click', async () => { S.codesDirty = false; S.sel = -1; await loadLevel(); renderCodes(); });

  async function emuToggle(i, on) {
    const c = S.codes[i]; if (!c) return;
    let key;
    if (hasSector(c.type)) key = 's:' + S.codes.slice(0, i).filter((x) => hasSector(x.type)).length;
    else key = `b:${S.level}:${i}`;
    try {
      await fetch('/play/' + S.level);
      const r = await api('POST', '/api/state/' + S.level, { action: 'toggleCode', key, on });
      toast(on ? 'Код введён в эмуляторе' : 'Код откачен в эмуляторе', 'ok');
      if (S.tab === 'emu') renderEmu(); else if (r.redirect) { /* переход уровня — увидим в эмуляторе */ }
    } catch (e) { toast('Эмулятор: ' + String(e.message || e) + ' (сохраните codes.yml — эмулятор читает файлы)', 'err'); }
  }

  // ---------------------------------------------------------------- редактор
  const ed = $('#editor'), gutter = $('#gutter');
  function renderEditor() {
    const d = S.data;
    const paths = d ? { body: d.files.body || (d.files.dir + '/task.html'), conf: d.files.conf, codes: d.files.codes } : {};
    $('#editorPath').textContent = paths[S.file] || '';
    if (document.activeElement !== ed) ed.value = S.raw[S.file] || '';
    updateGutter();
    $('#editorHint').textContent = { body: 'HTML тела задания. CSS — через <style>@import url("{{design.css}}")</style>: голый <link> движок вырежет.', conf: 'YAML настроек уровня; проверяется при сохранении.', codes: 'YAML кодов; проверяется при сохранении. Удобнее — вкладка «Коды».' }[S.file];
    renderConf();
  }
  function updateGutter() {
    const n = (ed.value.match(/\n/g) || []).length + 1;
    let s = ''; for (let i = 1; i <= n; i++) s += i + '\n';
    gutter.textContent = s; gutter.scrollTop = ed.scrollTop;
  }
  ed.addEventListener('input', () => { S.raw[S.file] = ed.value; S.rawDirty[S.file] = ed.value !== (S.data && S.data.raw ? S.data.raw[S.file] || '' : ''); updateDirty(); updateGutter(); });
  ed.addEventListener('scroll', () => { gutter.scrollTop = ed.scrollTop; });
  ed.addEventListener('keyup', updatePos); ed.addEventListener('click', updatePos);
  ed.addEventListener('keydown', (e) => {
    if (e.key === 'Tab') { e.preventDefault(); insertAtCursor('  '); }
  });
  function updatePos() { const before = ed.value.slice(0, ed.selectionStart); const line = (before.match(/\n/g) || []).length + 1; const col = before.length - before.lastIndexOf('\n'); $('#editorPos').textContent = `стр. ${line}, кол. ${col}`; }
  function insertAtCursor(text) {
    const s = ed.selectionStart, e = ed.selectionEnd;
    ed.value = ed.value.slice(0, s) + text + ed.value.slice(e);
    ed.selectionStart = ed.selectionEnd = s + text.length; ed.focus();
    ed.dispatchEvent(new Event('input'));
  }
  $$('.filetabs button[data-file]').forEach((b) => b.addEventListener('click', () => { S.file = b.dataset.file; $$('.filetabs button[data-file]').forEach((x) => x.classList.toggle('active', x === b)); ed.value = S.raw[S.file] || ''; renderEditor(); }));
  async function saveRaw() {
    if (!S.level) return;
    try {
      await api('PUT', `/api/ui/level/${S.level}/raw/${S.file}`, ed.value, true);
      S.rawDirty[S.file] = false; toast(({ body: 'task.html', conf: 'conf.yml', codes: 'codes.yml' })[S.file] + ' сохранён', 'ok');
      const keep = ed.value; await loadState(); await loadLevel(); S.raw[S.file] = keep; renderEditor();
    } catch (e) { toast(String(e.message || e), 'err'); }
  }
  $('#btnEditorSave').addEventListener('click', saveRaw);
  document.addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); if (S.tab === 'editor') saveRaw(); else if (S.tab === 'codes') $('#btnCodesSave').click(); } });

  // conf-форма
  function renderConf() {
    const c = S.conf || {};
    $('#cfName').value = c.name || ''; $('#cfComment').value = c.comment || '';
    $('#cfAutopass').value = c.autopass || ''; $('#cfPenalty').value = c.autopassPenalty || '';
    $('#cfSectors').value = c.sectorsToClose || ''; $('#cfClean').checked = !!c.clean;
    $('#hints').innerHTML = (c.hints || []).map((h, i) => `<div class="hint"><input class="time mono" type="number" min="0" value="${h.time || 0}" data-h="${i}" data-k="time" title="секунда появления"><textarea data-h="${i}" data-k="text">${esc(h.text || '')}</textarea><button data-hdel="${i}" title="Удалить">✕</button></div>`).join('') || '<div class="muted">нет</div>';
    $('#penalties').innerHTML = (c.penaltyHints || []).map((h, i) => `<div class="hint pen"><div class="col"><div class="row"><input class="time mono" type="number" min="0" value="${h.time || 0}" data-p="${i}" data-k="time" title="секунда доступности"><input class="time mono" type="number" min="0" value="${h.penalty || 0}" data-p="${i}" data-k="penalty" title="штраф, сек"><input data-p="${i}" data-k="comment" placeholder="комментарий-подтверждение" value="${esc(h.comment || '')}" style="flex:1"></div><textarea data-p="${i}" data-k="text">${esc(h.text || '')}</textarea></div><button data-pdel="${i}" title="Удалить">✕</button></div>`).join('') || '<div class="muted">нет</div>';
    $$('#hints [data-h]').forEach((el) => el.addEventListener('input', () => { c.hints[+el.dataset.h][el.dataset.k] = el.dataset.k === 'time' ? +el.value : el.value; markConf(); }));
    $$('#hints [data-hdel]').forEach((b) => b.addEventListener('click', () => { c.hints.splice(+b.dataset.hdel, 1); markConf(true); }));
    $$('#penalties [data-p]').forEach((el) => el.addEventListener('input', () => { c.penaltyHints[+el.dataset.p][el.dataset.k] = ['time', 'penalty'].includes(el.dataset.k) ? +el.value : el.value; markConf(); }));
    $$('#penalties [data-pdel]').forEach((b) => b.addEventListener('click', () => { c.penaltyHints.splice(+b.dataset.pdel, 1); markConf(true); }));
  }
  function markConf(rerender) { S.confDirty = true; updateDirty(); if (rerender) renderConf(); }
  ['cfName', 'cfComment', 'cfAutopass', 'cfPenalty', 'cfSectors', 'cfClean'].forEach((id) => $('#' + id).addEventListener('input', () => {
    const c = S.conf || (S.conf = {});
    c.name = $('#cfName').value; c.comment = $('#cfComment').value;
    c.autopass = +$('#cfAutopass').value || 0; c.autopassPenalty = +$('#cfPenalty').value || 0;
    c.sectorsToClose = +$('#cfSectors').value || 0; c.clean = $('#cfClean').checked; markConf();
  }));
  $('#btnAddHint').addEventListener('click', () => { const c = S.conf || (S.conf = {}); (c.hints = c.hints || []).push({ time: 0, text: '' }); markConf(true); });
  $('#btnAddPenalty').addEventListener('click', () => { const c = S.conf || (S.conf = {}); (c.penaltyHints = c.penaltyHints || []).push({ time: 0, text: '', penalty: 60, comment: '' }); markConf(true); });
  $('#btnConfSave').addEventListener('click', async () => {
    try {
      const c = Object.assign({}, S.conf);
      c.hints = (c.hints || []).filter((h) => h.text); c.penaltyHints = (c.penaltyHints || []).filter((h) => h.text);
      await api('PUT', `/api/ui/level/${S.level}/conf`, c);
      S.confDirty = false; toast('conf.yml сохранён', 'ok'); await loadState(); await loadLevel(); if (S.file === 'conf') ed.value = S.raw.conf; renderEditor();
    } catch (e) { toast(String(e.message || e), 'err'); }
  });
  $('#btnConfReload').addEventListener('click', async () => { S.confDirty = false; await loadLevel(); renderConf(); });

  // ---------------------------------------------------------------- превью
  function previewURL() {
    const q = $$('[data-po]').map((i) => i.dataset.po + '=' + (i.checked ? 1 : 0)).join('&');
    return `/ui/preview/${S.level}?${q}&_=${Date.now()}`;
  }
  async function renderPreview() {
    if (!S.level) return;
    $('#previewTitle').textContent = 'Превью · уровень ' + S.level;
    $('#previewFrame').src = previewURL();
    try {
      const checks = await api('GET', `/api/ui/preview/${S.level}/checks`);
      $('#checks').innerHTML = checks.map((c) => `<div class="chk ${c.level}">${esc(c.text)}</div>`).join('');
    } catch (e) { $('#checks').innerHTML = `<div class="chk error">${esc(e.message || e)}</div>`; }
  }
  $$('[data-po]').forEach((i) => i.addEventListener('change', renderPreview));
  $('#btnPreviewReload').addEventListener('click', renderPreview);
  $$('.seg [data-vw]').forEach((b) => b.addEventListener('click', () => { $$('.seg [data-vw]').forEach((x) => x.classList.toggle('active', x === b)); $('#previewFrame').parentElement.classList.toggle('phone', b.dataset.vw === '390'); }));

  // ---------------------------------------------------------------- эмулятор
  async function renderEmu() {
    if (!S.state) return;
    try { if (S.level) await fetch('/play/' + S.level); } catch (e) { /* ignore */ }
    $('#emuFrame').src = S.state.playPath + '?_=' + Date.now();
    try {
      const names = await api('GET', '/api/ui/snapshots');
      const sel = $('#snapSelect');
      sel.innerHTML = '<option value="">снимок…</option>' + names.map((n) => `<option>${esc(n)}</option>`).join('');
    } catch (e) { /* ignore */ }
  }
  $('#btnEmuReload').addEventListener('click', () => { $('#emuFrame').src = S.state.playPath + '?_=' + Date.now(); });
  $$('.seg [data-ew]').forEach((b) => b.addEventListener('click', () => { $$('.seg [data-ew]').forEach((x) => x.classList.toggle('active', x === b)); $('#emuFrame').parentElement.classList.toggle('phone', b.dataset.ew !== '100%'); }));
  $('#btnDiff').addEventListener('click', async () => {
    const name = $('#snapSelect').value; if (!name) { toast('Выберите снимок', 'err'); return; }
    const p = $('#diffPanel'); p.hidden = false; p.textContent = 'сравниваю…';
    try {
      const r = await api('GET', '/api/ui/diff/' + encodeURIComponent(name));
      if (r.equal) { p.innerHTML = `<b class="add">Совпадает побайтово</b> (после маскировки динамического): ${r.realLines} строк`; return; }
      p.innerHTML = `<b class="del">Расхождений: ${r.changes}</b> · real ${r.realLines} / emu ${r.emuLines} строк\n\n` + r.lines.map((l) => `<span class="${l.kind === '+' ? 'add' : l.kind === '-' ? 'del' : 'ctx'}">${l.kind} ${esc(l.text)}</span>`).join('\n');
    } catch (e) { p.textContent = 'Ошибка: ' + (e.message || e); }
  });

  // ---------------------------------------------------------------- старт
  (async function init() {
    try { await loadState(); await loadLevel(); } catch (e) { toast(String(e.message || e), 'err'); }
    setTab((location.hash || '#commands').slice(1));
    // если задание уже идёт (страницу перезагрузили) — подхватить лог
    if (S.state && S.state.job && !S.state.job.done) { S.job = S.state.job.id; S.jobFrom = 0; $('#log').textContent = ''; pollJob(); }
    setInterval(async () => { if (S.tab === 'commands' && !S.job) { try { await loadState(); } catch (e) { /* ignore */ } } }, 15000);
  })();
})();
