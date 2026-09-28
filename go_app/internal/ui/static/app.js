// app.js — веб-интерфейс zapolnyaka: вкладки Команды / Коды / Уровень / Редактор / Визуальный / Превью / Эмулятор.
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
    file: 'body', raw: {}, rawDirty: {}, orig: {}, open: [], mdKey: null, conf: null, confDirty: false,
    ex: { level: null, game: null, loadedFor: -1 }, exOpen: new Set(),
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
    if (!st.game) sel.innerHTML = '<option value="">— игры нет: создайте на вкладке «Команды» —</option>' + sel.innerHTML;
    $('#cardNewGame').classList.toggle('attention', !st.game);
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
      <div class="level${l.number === S.level && S.level ? ' active' : ''}${l.disabled ? ' disabled' : ''}" data-level="${l.number}" data-conf="${esc(l.conf)}" title="${esc(l.error || (l.disabled ? 'выключен в game.yml (закомментирован) — не заливается' : l.dir))}">
        <input type="checkbox" data-sel="${l.number}"${S.selected.has(l.number) ? ' checked' : ''} aria-label="Выбрать уровень ${l.number}" ${S.tab === 'commands' && !l.disabled ? '' : 'hidden'}>
        <span class="num">${l.number ? String(l.number).padStart(2, '0') : '??'}</span>
        <span class="name">${esc(l.name || l.dir)}</span>
        <span class="meta">${l.codes ? l.codes + ' код.' : ''}</span>
        <span class="sw${l.disabled ? '' : ' on'}" data-toggle="${l.disabled ? 1 : 0}" title="${l.disabled ? 'Включить (раскомментировать в game.yml)' : 'Выключить (закомментировать в game.yml)'}"></span>
        <span class="st${l.error ? ' err' : ''}"></span>
      </div>`).join('') || `<div class="muted" style="padding:8px">${S.state.game ? 'В игре нет уровней — добавьте «+».' : 'Игры ещё нет — создайте её в карточке «Создать игру» на вкладке «Команды».'}</div>`;
    $$('.level', box).forEach((el) => el.addEventListener('click', (e) => {
      if (e.target.matches('input[data-sel]')) { const n = +e.target.dataset.sel; if (e.target.checked) S.selected.add(n); else S.selected.delete(n); renderSelectedChip(); return; }
      if (e.target.matches('.sw')) { toggleLevel(el.dataset.conf, e.target.dataset.toggle === '1'); return; }
      selectLevel(+el.dataset.level);
    }));
    renderSelectedChip();
  }

  async function toggleLevel(conf, enable) {
    try {
      await api('POST', '/api/ui/level/enabled', { conf, enabled: enable });
      toast((enable ? 'Включён: ' : 'Выключен: ') + conf + ' (game.yml)', 'ok');
      await loadState();
      if (S.tab === 'emu') renderEmu();
    } catch (e) { toast(String(e.message || e), 'err'); }
  }
  const enabledCount = () => (S.state ? S.state.levels.filter((l) => !l.disabled).length : 0);

  function renderSelectedChip() {
    const arr = Array.from(S.selected).sort((a, b) => a - b);
    $('#selectedChip').textContent = arr.length ? arr.map((n) => String(n).padStart(2, '0')).join(', ') : 'все';
  }

  function renderAssets() {
    $('#assets').innerHTML = S.state.assets.map((a) => `<div draggable="true" data-asset="${esc(a.name)}" title="${esc(a.name)} · ${a.size} байт${a.uploaded ? ' · на сервере ' + esc(a.uploaded) : ''}"><span>${esc(a.name)}</span><span class="${a.uploaded ? 'up' : 'no'}">${a.uploaded ? 'залит' : 'не залит'}</span><span class="del" data-del="${esc(a.name)}" title="Удалить файл из папки ассетов">✕</span></div>`).join('') || '<div class="muted">папка ассетов пуста — «+» добавит файлы</div>';
    $('#assetsDir').textContent = S.state.assetsDir || '';
    $$('#assets .del').forEach((b) => b.addEventListener('click', async () => {
      if (!confirm('Удалить файл «' + b.dataset.del + '» из папки ассетов?')) return;
      try { await api('DELETE', '/api/ui/assets/' + encodeURIComponent(b.dataset.del)); toast('Удалён: ' + b.dataset.del, 'ok'); await loadState(); }
      catch (e) { toast(String(e.message || e), 'err'); }
    }));
    $('#assetButtons').innerHTML = S.state.assets.map((a) => `<button class="mono small" data-insert="{{${esc(a.name)}}}">{{${esc(a.name)}}}</button>`).join('') || '<span class="muted">нет ассетов</span>';
    $$('#assetButtons button').forEach((b) => b.addEventListener('click', () => {
      const name = b.dataset.insert.slice(2, -2);
      insertAtCursor(keyKind(S.file) === 'md' ? assetLink(name) : b.dataset.insert);
    }));
  }
  $('#btnAddAsset').addEventListener('click', () => $('#assetFiles').click());
  $('#assetFiles').addEventListener('change', async (e) => {
    const files = Array.from(e.target.files || []); if (!files.length) return;
    const fd = new FormData(); files.forEach((f) => fd.append('files', f, f.name));
    try {
      const r = await fetch('/api/ui/assets', { method: 'POST', body: fd });
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || r.statusText);
      toast('Добавлено: ' + data.saved.join(', ') + ' — для игры выполните «Залить ассеты»', 'ok');
      await loadState();
    } catch (err) { toast(String(err.message || err), 'err'); }
    e.target.value = '';
  });

  async function selectLevel(n) {
    await flushAutosave();
    const levelDirty = Object.keys(S.rawDirty).some((k) => S.rawDirty[k] && !(keyInfo(k) && keyInfo(k).scope === 'game'));
    if (S.codesDirty || levelDirty || S.confDirty) {
      if (!confirm('Есть несохранённые изменения. Переключить уровень и потерять их?')) return;
    }
    const gameDirty = {};
    Object.keys(S.rawDirty).forEach((k) => { if (keyInfo(k) && keyInfo(k).scope === 'game') gameDirty[k] = S.rawDirty[k]; });
    S.level = n; S.codesDirty = false; S.rawDirty = gameDirty; S.confDirty = false; S.sel = -1;
    // вкладки файлов прежнего уровня закрываются, notes/ игры остаются открытыми
    S.open.filter((o) => o.scope === 'level').forEach((o) => { delete S.raw[o.key]; delete S.orig[o.key]; dropSession(o.key); MD.forget(o.key); });
    S.open = S.open.filter((o) => o.scope !== 'level');
    MD.forget('notes');
    if (keyInfo(S.file) && !S.open.some((o) => o.key === S.file)) S.file = 'notes';
    renderLevels();
    await loadLevel();
    refreshTab();
  }

  async function loadLevel() {
    if (!S.level) { S.data = null; return; }
    try {
      S.data = await api('GET', '/api/ui/level/' + S.level);
      S.codes = JSON.parse(JSON.stringify(S.data.codes || []));
      // открытые файлы обозревателя и несохранённые правки не затираются
      const keep = {};
      Object.keys(S.raw).forEach((k) => { if (keyInfo(k) || S.rawDirty[k]) keep[k] = S.raw[k]; });
      S.raw = Object.assign({ body: '', conf: '', codes: '', notes: '' }, S.data.raw || {}, keep);
      S.conf = JSON.parse(JSON.stringify(S.data.conf || {}));
      if (S.data.error) toast('Ошибка в файлах уровня: ' + S.data.error, 'err');
    } catch (e) { S.data = null; toast(String(e.message || e), 'err'); }
    updateDirty();
  }

  function updateDirty() {
    const d = [];
    if (S.codesDirty) d.push('codes.yml');
    if (S.confDirty) d.push('conf.yml');
    Object.keys(S.rawDirty).forEach((k) => { if (S.rawDirty[k] && !MDAUTO.timers[k]) d.push(keyName(k)); });
    $('#stDirty').textContent = d.length ? 'не сохранено: ' + Array.from(new Set(d)).join(', ') : '';
    $('#stDirty').className = d.length ? 'dirty' : '';
    $$('.filetabs button[data-file]').forEach((b) => b.classList.toggle('dirty', !!S.rawDirty[b.dataset.file]));
    $$('#openTabs [data-open]').forEach((b) => b.classList.toggle('dirty', !!S.rawDirty[b.dataset.open]));
  }

  // ---------------------------------------------------------------- вкладки
  function setTab(tab) {
    if (S.tab === 'visual' && tab !== 'visual') veFlush();
    S.tab = tab;
    $$('#tabs a').forEach((a) => a.classList.toggle('active', a.dataset.tab === tab));
    $$('.tab').forEach((s) => s.classList.toggle('active', s.id === 'tab-' + tab));
    $$('#levels input[data-sel]').forEach((i) => { i.hidden = tab !== 'commands' || i.closest('.level').classList.contains('disabled'); });
    refreshTab();
  }
  function refreshTab() {
    if (S.tab === 'codes') renderCodes();
    if (S.tab === 'level') renderConf();
    if (S.tab === 'editor') renderEditor();
    if (S.tab === 'visual') renderVisual();
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
      const total = (S.selected.size && $('#onlySelected').checked) ? S.selected.size : enabledCount();
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
    if (!e.target.value) return;
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

  // ---------------------------------------------------------------- редактор (Ace + md)
  // Файлы редактора — ключи: стандартные body | conf | codes | notes и открытые из
  // обозревателя «f:<scope>:<путь>» (scope: level — папка уровня, game — notes/ игры).
  // Текстовые файлы — в Ace (по сессии на файл: своя история отмены; режим по
  // расширению; автодополнение: теги из режима html, ключи YAML и типы кодов — свои,
  // {{ассеты}} — везде). .md — в CodeMirror 6 с Live Preview (static/vendor/mdedit,
  // исходники — ../mdedit) и автосохранением. Картинки — просмотрщик.
  ace.config.set('basePath', '/ui/static/vendor/ace');
  const Range = ace.require('ace/range').Range;
  const langTools = ace.require('ace/ext/language_tools');
  const E = ace.edit('editor', {
    theme: 'ace/theme/one_dark', fontFamily: 'JetBrains Mono, Consolas, Cascadia Mono, monospace', fontSize: '13px',
    showPrintMargin: false, tabSize: 2, useSoftTabs: true, wrap: false, highlightActiveLine: true, scrollPastEnd: 0.2,
    enableBasicAutocompletion: true, enableLiveAutocompletion: true, enableSnippets: true, fixedWidthGutter: true,
  });
  const STD = { body: 'task.html', conf: 'conf.yml', codes: 'codes.yml', notes: 'notes.md' };
  const isStd = (k) => Object.prototype.hasOwnProperty.call(STD, k);
  const extOf = (p) => { const m = /\.([^./]+)$/.exec(p || ''); return m ? m[1].toLowerCase() : ''; };
  const IMG_EXT = /^(png|jpe?g|gif|webp|svg|bmp|ico|avif)$/;
  const ACE_MODES = { html: 'html', htm: 'html', yml: 'yaml', yaml: 'yaml', json: 'json', css: 'css', js: 'javascript', mjs: 'javascript' };
  // keyInfo: {scope, path} для файла из обозревателя.
  const keyInfo = (k) => { const m = /^f:(level|game):(.*)$/.exec(k); return m ? { scope: m[1], path: m[2] } : null; };
  const fileKey = (scope, path) => 'f:' + scope + ':' + path;
  const stdPath = (k) => { const d = S.data; if (!d) return ''; return { body: d.files.body || (d.files.dir + '/task.html'), conf: d.files.conf, codes: d.files.codes, notes: d.files.notes }[k] || ''; };
  const keyName = (k) => (isStd(k) ? (stdPath(k).split('/').pop() || STD[k]) : (keyInfo(k) || { path: k }).path.split('/').pop());
  const keyFile = (k) => (isStd(k) ? stdPath(k) || STD[k] : keyInfo(k).path);
  const keyKind = (k) => { const e = extOf(keyFile(k)); return e === 'md' || e === 'markdown' ? 'md' : IMG_EXT.test(e) ? 'image' : 'text'; };
  const origOf = (k) => (isStd(k) ? ((S.data && S.data.raw && S.data.raw[k]) || '') : (S.orig[k] || ''));
  const sessions = {};
  let edSyncing = false;
  function edSession(key) {
    if (!sessions[key]) {
      const s = ace.createEditSession(S.raw[key] || '', 'ace/mode/' + (ACE_MODES[extOf(keyFile(key))] || 'text'));
      s.setTabSize(2); s.setUseSoftTabs(true); s.setUseWrapMode(false);
      s.on('change', () => {
        if (edSyncing) return;
        const v = s.getValue();
        S.raw[key] = v; S.rawDirty[key] = v !== origOf(key); updateDirty();
      });
      sessions[key] = s;
    }
    return sessions[key];
  }
  // edSet кладёт текст в сессию без пометки «изменено» (синхронизация с S.raw).
  function edSet(key, text) {
    const s = edSession(key);
    if (s.getValue() === text) return;
    edSyncing = true;
    try { s.setValue(text); s.selection.moveCursorFileStart(); s.selection.clearSelection(); } finally { edSyncing = false; }
  }
  function dropSession(key) { if (sessions[key]) { sessions[key].destroy && sessions[key].destroy(); delete sessions[key]; } }

  // --- md-редактор (CodeMirror 6, Live Preview)
  const MD = window.MdEdit.create($('#mdeditor'), {
    placeholder: 'Заметки автора: идеи, ответы, ссылки. Картинки и ассеты — перетащи сюда или вставь из буфера (Ctrl+V).',
    resolve: (url, wiki) => mdResolve(url, wiki),
    onOpenLink: (href, wiki) => mdOpenLink(href, wiki),
    onChange: (text) => {
      const key = S.mdKey; if (!key) return;
      S.raw[key] = text; S.rawDirty[key] = text !== origOf(key); updateDirty();
      mdAutosave(key);
    },
    onDrop: (dt) => mdDrop(dt),
    onPasteFiles: (files) => mdPasteFiles(files),
  });
  const MDAUTO = { timers: {} };
  // mdAutosave: md-файлы сохраняются сами через 800 мс после правки (notes.md
  // появляется на диске при первой правке, простое открытие файл не создаёт).
  function mdAutosave(key) {
    clearTimeout(MDAUTO.timers[key]);
    MDAUTO.timers[key] = setTimeout(() => { delete MDAUTO.timers[key]; if (S.rawDirty[key]) saveFile(key, S.raw[key] || '', true); }, 800);
  }
  async function flushAutosave(keepalive) {
    const keys = Object.keys(MDAUTO.timers);
    keys.forEach((k) => clearTimeout(MDAUTO.timers[k]));
    MDAUTO.timers = {};
    await Promise.all(keys.filter((k) => S.rawDirty[k]).map((k) => saveFile(k, S.raw[k] || '', true, keepalive)));
  }
  window.addEventListener('pagehide', () => { flushAutosave(true); });

  function currentText() { const k = keyKind(S.file); return k === 'md' ? MD.getDoc() : k === 'image' ? null : E.getValue(); }
  const edValue = currentText;

  function renderEditor() {
    if (!isStd(S.file) && !S.open.some((o) => o.key === S.file)) S.file = 'body';
    const key = S.file, kind = keyKind(key);
    renderFileTabs();
    $('#editor').hidden = kind !== 'text';
    $('#mdeditor').hidden = kind !== 'md';
    $('#imgview').hidden = kind !== 'image';
    $('#assetbar').hidden = kind === 'image';
    $('#btnEditorSave').hidden = kind === 'image';
    $('#editorKeys').textContent = kind === 'md' ? 'Live Preview · Ctrl/Cmd+клик — открыть ссылку · автосохранение' : kind === 'text' ? 'Ctrl+Space — подсказки · {{ — ассеты · Ctrl+F — поиск' : '';
    const info = keyInfo(key);
    // путь обрезается слева (direction:rtl), LRM не даёт bidi переставить «/» в конец
    $('#editorPath').textContent = '\u200E' + (isStd(key) ? stdPath(key) : ((S.ex[info.scope] && S.ex[info.scope].root) || '') + '/' + info.path) + '\u200E';
    const noLevel = isStd(key) && !S.level;
    if (kind === 'text') {
      ['body', 'conf', 'codes'].forEach((f) => edSet(f, S.raw[f] || ''));
      if (E.session !== edSession(key)) E.setSession(edSession(key));
      E.setReadOnly(noLevel);
      E.resize(true);
      updatePos();
    } else if (kind === 'md') {
      S.mdKey = null; // пока открываем — изменения не наши
      MD.open(key, S.raw[key] || '');
      S.mdKey = key;
      MD.setReadOnly(noLevel);
      $('#editorPos').textContent = '';
    } else {
      renderImageView(info);
    }
    $('#editorHint').textContent = ({
      body: 'HTML тела задания. CSS — через <style>@import url("{{design.css}}")</style>: голый <link> движок вырежет.',
      conf: 'YAML настроек уровня; проверяется при сохранении. Удобнее — вкладка «Уровень».',
      codes: 'YAML кодов; проверяется при сохранении. Удобнее — вкладка «Коды».',
      notes: 'Заметки автора к уровню — только для тебя, на en.cx не заливаются.',
    })[key] || (info && info.scope === 'game' ? 'Общая папка игры notes/ — на en.cx не заливается.' : 'Файл папки уровня — на en.cx не заливается (для задания — «⋯ → в ассеты»).');
    ensureExplorer();
  }
  function renderImageView(info) {
    const src = fsURL(gameRel(info.scope, info.path));
    const ent = exEntry(info.scope, info.path);
    $('#imgview').innerHTML = `<div class="imgview-pic"><img src="${esc(src)}" alt=""></div>
      <div class="imgview-foot"><span class="mono">${esc(info.path)}</span><span class="muted">${ent ? fmtSize(ent.size) : ''}</span><span class="grow"></span>
      <a class="btn" href="${esc(src)}" target="_blank">Открыть</a><button class="small" data-toassets>В ассеты</button></div>`;
    $('#imgview [data-toassets]').addEventListener('click', () => exToAssets(info.scope, info.path));
  }
  function renderFileTabs() {
    $$('.filetabs button[data-file]').forEach((b) => { b.classList.toggle('active', b.dataset.file === S.file); b.disabled = !S.level; });
    $('#openTabs').innerHTML = S.open.map((o) => `<button data-open="${esc(o.key)}" class="${o.key === S.file ? 'active' : ''}${S.rawDirty[o.key] ? ' dirty' : ''}" title="${esc((o.scope === 'game' ? 'notes/' : '') + o.path)}"><span class="nm">${esc(o.path.split('/').pop())}</span><span class="x" data-close="${esc(o.key)}" title="Закрыть">✕</span></button>`).join('');
    $$('#openTabs [data-open]').forEach((b) => b.addEventListener('click', (e) => {
      if (e.target.dataset.close) { closeFile(e.target.dataset.close); return; }
      switchFile(b.dataset.open);
    }));
  }
  function switchFile(key) {
    S.file = key; renderEditor();
    const k = keyKind(key); if (k === 'md') MD.focus(); else if (k === 'text') E.focus();
  }
  async function closeFile(key) {
    if (MDAUTO.timers[key]) await flushAutosave();
    if (S.rawDirty[key] && !confirm('Файл «' + keyName(key) + '» не сохранён. Закрыть без сохранения?')) return;
    S.open = S.open.filter((o) => o.key !== key);
    delete S.raw[key]; delete S.rawDirty[key]; delete S.orig[key]; dropSession(key); MD.forget(key);
    if (S.file === key) S.file = S.open.length ? S.open[S.open.length - 1].key : 'body';
    updateDirty(); renderEditor();
  }
  function updatePos() { const p = E.getCursorPosition(); $('#editorPos').textContent = `стр. ${p.row + 1}, кол. ${p.column + 1}`; }
  E.on('changeSelection', updatePos);
  function insertAtCursor(text) {
    if (keyKind(S.file) === 'md') { MD.insert(text); return; }
    E.insert(text); E.focus();
  }
  $$('.filetabs button[data-file]').forEach((b) => b.addEventListener('click', () => switchFile(b.dataset.file)));

  // --- автодополнение: {{ассеты}} во всех файлах
  const assetCompleter = {
    getCompletions(editor, session, pos, prefix, cb) {
      const line = session.getLine(pos.row).slice(0, pos.column);
      if (!/\{\{\s*[^{}]*$/.test(line)) return cb(null, []);
      cb(null, (S.state ? S.state.assets : []).map((a) => ({ caption: '{{' + a.name + '}}', value: a.name, meta: a.uploaded ? 'ассет' : 'ассет (не залит)', score: 2000, name: a.name, completer: assetCompleter })));
    },
    insertMatch(editor, data) {
      const pos = editor.getCursorPosition(), line = editor.session.getLine(pos.row).slice(0, pos.column);
      const m = /\{\{\s*([^{}]*)$/.exec(line);
      if (m) editor.session.replace(new Range(pos.row, pos.column - m[1].length, pos.row, pos.column), '');
      const after = editor.session.getLine(pos.row).slice(editor.getCursorPosition().column);
      editor.insert(data.name + (after.startsWith('}}') ? '' : '}}'));
    },
  };
  // --- автодополнение: ключи conf.yml / codes.yml и типы кодов
  const CONF_KEYS = [
    ['level', 'номер уровня (обязательно)'], ['name', 'название уровня в админке'], ['comment', 'комментарий к уровню в админке'],
    ['codes', 'файл кодов, напр. codes.yml'], ['body', 'файл тела задания, напр. task.html'], ['clean', 'true — очистить уровень перед заливкой'],
    ['autopass', 'автопереход через N секунд (0 — выкл)'], ['autopassPenalty', 'штраф при автопереходе, сек'],
    ['sectorsToClose', 'условие прохождения: сколько секторов закрыть (по умолчанию все)'],
    ['hints', 'обычные подсказки: список {time, text}'], ['penaltyHints', 'штрафные подсказки: список {time, text, penalty, comment}'],
    ['time', 'секунда уровня, когда подсказка откроется'], ['text', 'текст подсказки (HTML, {{ассеты}})'],
    ['penalty', 'штраф за штрафную подсказку, сек'],
  ];
  const CODE_KEYS = [
    ['type', 'сектор | бонус | штраф | секторбонус | секторштраф'], ['answers', 'список ответов (кодов)'],
    ['sectorName', 'имя сектора — видно игроку сразу'], ['bonusName', 'имя бонуса — видно игроку сразу'],
    ['time', 'время бонуса/штрафа, сек (обязательно для бонусных типов)'], ['task', 'задание бонуса — видно до ввода'],
    ['help', 'текст после ввода кода (HTML, <script>, {{ассеты}})'], ['levels', 'уровни бонуса: "1-10", ">10", "2,4,6-8", "*"'],
  ];
  const CODE_TYPES = ['сектор', 'бонус', 'штраф', 'секторбонус', 'секторштраф'];
  const yamlCompleter = {
    getCompletions(editor, session, pos, prefix, cb) {
      if (S.file !== 'conf' && S.file !== 'codes') return cb(null, []);
      const line = session.getLine(pos.row), before = line.slice(0, pos.column - prefix.length);
      if (S.file === 'codes' && /^\s*(-\s+)?type:\s*$/.test(before)) {
        return cb(null, CODE_TYPES.map((t) => ({ caption: t, value: t, meta: 'тип кода', score: 1500 })));
      }
      if (!/^\s*(-\s+)?$/.test(before)) return cb(null, []);
      const keys = S.file === 'conf' ? CONF_KEYS : CODE_KEYS;
      cb(null, keys.map(([k, doc]) => ({ caption: k, value: k + ': ', meta: 'ключ', score: 1000, docHTML: '<b>' + k + '</b> — ' + esc(doc) })));
    },
  };
  // Слова из текущего файла — только в HTML: в YAML они дублируют подсказки ключей.
  const localWordsCompleter = { getCompletions(editor, session, pos, prefix, cb) { if (S.file !== 'body') return cb(null, []); langTools.textCompleter.getCompletions(editor, session, pos, prefix, cb); } };
  E.completers = [assetCompleter, yamlCompleter, langTools.snippetCompleter, langTools.keyWordCompleter, localWordsCompleter];
  // «{{» сразу открывает список ассетов
  E.commands.on('afterExec', (e) => {
    if (e.command.name !== 'insertstring' || e.args !== '{') return;
    const pos = E.getCursorPosition();
    if (/\{\{$/.test(E.session.getLine(pos.row).slice(0, pos.column))) E.execCommand('startAutocomplete');
  });

  // saveFile сохраняет файл редактора. quiet — автосохранение: без тоста и без
  // перечитывания уровня (чтобы не сбить набор текста).
  async function saveFile(key, text, quiet, keepalive) {
    const info = keyInfo(key);
    if (isStd(key) && !S.level) return false;
    const level = S.level;
    try {
      const url = info ? `/api/ui/file?scope=${info.scope}&n=${level || 0}&path=${encodeURIComponent(info.path)}` : `/api/ui/level/${level}/raw/${key}`;
      if (keepalive) { fetch(url, { method: 'PUT', body: text, keepalive: true, headers: { 'Content-Type': 'text/plain; charset=utf-8' } }); return true; }
      const existed = info ? !!exEntry(info.scope, info.path) : !!origOf(key);
      await api('PUT', url, text, true);
      if (level !== S.level && (isStd(key) || info.scope === 'level')) return true; // уровень сменили, пока сохраняли
      if (isStd(key)) { if (S.data) S.data.raw[key] = text; } else S.orig[key] = text;
      S.rawDirty[key] = (S.raw[key] == null ? text : S.raw[key]) !== text;
      updateDirty();
      if (!existed && (text || info)) loadExplorer();
      if (quiet) return true;
      toast(keyName(key) + ' сохранён', 'ok');
      if (key === 'body' || key === 'conf' || key === 'codes') {
        await loadState(); await loadLevel(); S.raw[key] = text;
        if (S.tab === 'editor') renderEditor();
      }
      return true;
    } catch (e) { toast(keyName(key) + ': ' + String(e.message || e), 'err'); return false; }
  }
  const saveRaw = () => { const t = currentText(); if (t != null) saveFile(S.file, t); };
  $('#btnEditorSave').addEventListener('click', saveRaw);
  document.addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); if (S.tab === 'editor') saveRaw(); else if (S.tab === 'visual') veSave(); else if (S.tab === 'codes') $('#btnCodesSave').click(); else if (S.tab === 'level') $('#btnConfSave').click(); } });

  // ---------------------------------------------------------------- файловый обозреватель
  // Два корня: level — папка уровня, game — notes/ рядом с game.yml. Пути в API —
  // относительно корня; для картинок в md — относительно папки игры (/ui/fs/…).
  const fmtSize = (n) => (n < 1024 ? n + ' Б' : n < 1048576 ? (n / 1024).toFixed(0) + ' КБ' : (n / 1048576).toFixed(1) + ' МБ');
  const joinPath = (...parts) => {
    const out = [];
    parts.join('/').split('/').forEach((seg) => { if (!seg || seg === '.') return; if (seg === '..') out.pop(); else out.push(seg); });
    return out.join('/');
  };
  const dirOf = (p) => p.split('/').slice(0, -1).join('/');
  const fsURL = (rel) => '/ui/fs/' + rel.split('/').map(encodeURIComponent).join('/');
  const gameRel = (scope, path) => joinPath((S.ex[scope] && S.ex[scope].rel) || (scope === 'game' ? 'notes' : ''), path);
  const exEntry = (scope, path) => (S.ex[scope] ? S.ex[scope].entries.find((e) => e.path === path) : null);
  // scopeOf: путь от папки игры → {scope, path} или null (вне корней).
  function scopeOf(rel) {
    for (const scope of ['level', 'game']) {
      const r = S.ex[scope]; if (!r) continue;
      if (scope === 'level' && !S.level) continue;
      if (r.rel === '') return { scope, path: rel };
      if (rel === r.rel || rel.startsWith(r.rel + '/')) return { scope, path: rel.slice(r.rel.length + 1) };
    }
    return null;
  }
  // relLink — путь от папки fromDir до to (оба — от папки игры), для md-ссылок.
  function relLink(fromDir, to) {
    const a = fromDir ? fromDir.split('/') : [], b = to.split('/');
    let i = 0; while (i < a.length && i < b.length - 1 && a[i] === b[i]) i++;
    return '../'.repeat(a.length - i) + b.slice(i).join('/');
  }
  const mdURL = (p) => p.replace(/ /g, '%20').replace(/\(/g, '%28').replace(/\)/g, '%29');
  // mdDir — папка текущего md-файла (от папки игры).
  function mdDir() {
    const k = S.mdKey || S.file;
    if (k === 'notes') return (S.ex.level && S.ex.level.rel) || '';
    const info = keyInfo(k); return info ? dirOf(gameRel(info.scope, info.path)) : '';
  }
  function allFiles() {
    const out = [];
    ['level', 'game'].forEach((scope) => { const r = S.ex[scope]; if (r && (scope !== 'level' || S.level)) r.entries.forEach((e) => { if (!e.dir) out.push(joinPath(r.rel, e.path)); }); });
    return out;
  }
  // findWiki — цель [[вики-ссылки]] как в Obsidian: рядом с заметкой, от корня
  // уровня, от notes/, затем по имени файла где угодно.
  function findWiki(target) {
    const files = allFiles(), has = (p) => files.includes(p);
    target = target.replace(/^\/+/, '');
    const cands = [joinPath(mdDir(), target), joinPath((S.ex.level && S.ex.level.rel) || '', target), joinPath('notes', target)];
    for (const c of cands) if (has(c)) return c;
    const base = target.split('/').pop();
    return files.find((f) => f.split('/').pop() === base) || null;
  }
  function mdResolve(url, wiki) {
    if (!url) return '';
    const asset = /^\{\{\s*([^{}]+?)\s*\}\}$/.exec(url);
    if (asset) return '/assets/' + encodeURIComponent(asset[1]);
    if (/^(https?:|data:|blob:|\/\/)/i.test(url)) return url;
    let p = url.split('#')[0];
    try { p = decodeURIComponent(p); } catch (e) { /* как есть */ }
    if (wiki) { const f = findWiki(p); return f ? fsURL(f) : ''; }
    return fsURL(p.startsWith('/') ? joinPath(p) : joinPath(mdDir(), p));
  }
  async function mdOpenLink(href, wiki) {
    if (!href) return;
    if (/^(https?:|mailto:)/i.test(href)) { window.open(href, '_blank', 'noopener'); return; }
    let p = href.split('#')[0];
    try { p = decodeURIComponent(p); } catch (e) { /* как есть */ }
    let rel;
    if (wiki) {
      rel = findWiki(p) || (extOf(p) ? null : findWiki(p + '.md'));
      if (!rel) { // новая заметка рядом с текущей — файл появится при первой правке
        rel = joinPath(mdDir(), extOf(p) ? p : p + '.md');
        const sc = scopeOf(rel);
        if (!sc) { toast('Не найдено: ' + p, 'err'); return; }
        const key = fileKey(sc.scope, sc.path);
        if (!S.open.some((o) => o.key === key)) { S.open.push({ key, scope: sc.scope, path: sc.path }); S.raw[key] = ''; S.orig[key] = ''; }
        switchFile(key);
        return;
      }
    } else rel = p.startsWith('/') ? joinPath(p) : joinPath(mdDir(), p);
    const sc = scopeOf(rel);
    if (sc && exEntry(sc.scope, sc.path)) openExFile(sc.scope, sc.path);
    else window.open(fsURL(rel), '_blank', 'noopener');
  }
  // linkFor — md-ссылка на файл обозревателя из текущей заметки.
  function linkFor(scope, path) {
    const rel = relLink(mdDir(), gameRel(scope, path)), name = path.split('/').pop();
    return IMG_EXT.test(extOf(path)) ? `![](${mdURL(rel)})` : `[${name}](${mdURL(rel)})`;
  }
  const assetLink = (name) => (IMG_EXT.test(extOf(name)) ? `![]({{${name}}})` : `{{${name}}}`);
  // mdTarget — куда класть файлы, брошенные в заметку: в её папку.
  function mdTarget() {
    const k = S.mdKey || S.file;
    if (k === 'notes') return { scope: 'level', dir: '' };
    const info = keyInfo(k); return info ? { scope: info.scope, dir: dirOf(info.path) } : null;
  }
  function mdDrop(dt) {
    if (!dt) return null;
    const zf = dt.getData('application/x-zp-file');
    if (zf) { const f = JSON.parse(zf); return f.dir ? null : linkFor(f.scope, f.path); }
    const za = dt.getData('application/x-zp-asset');
    if (za) return assetLink(za);
    if (dt.files && dt.files.length) return mdUploadAndLink([...dt.files]);
    return null;
  }
  function mdPasteFiles(files) {
    const imgs = files.filter((f) => /^image\//.test(f.type));
    if (!imgs.length) return null;
    const d = new Date(), pad = (n) => String(n).padStart(2, '0');
    const stamp = `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
    return mdUploadAndLink(imgs.map((f) => ({ file: f, name: f.name && f.name !== 'image.png' ? f.name : `Pasted image ${stamp}.${(f.type.split('/')[1] || 'png').replace('jpeg', 'jpg').replace('svg+xml', 'svg')}` })));
  }
  async function mdUploadAndLink(files) {
    const t = mdTarget();
    if (!t) return null;
    const saved = await exUpload(t.scope, t.dir, files);
    return saved.map((p) => linkFor(t.scope, p)).join('\n');
  }

  async function loadExplorer() {
    const want = S.level;
    const get = async (scope) => { try { return await api('GET', `/api/ui/files?scope=${scope}&n=${want || 0}`); } catch (e) { return null; } };
    const [lv, gm] = await Promise.all([want ? get('level') : null, get('game')]);
    if (want !== S.level) return;
    S.ex.level = lv; S.ex.game = gm || { rel: 'notes', root: '', entries: [] }; S.ex.loadedFor = want;
    renderExplorer();
    MD.refresh();
    if (S.tab === 'editor' && keyKind(S.file) === 'image') renderImageView(keyInfo(S.file));
  }
  function ensureExplorer() { if (S.ex.loadedFor !== S.level || !S.ex.game) loadExplorer(); else renderExplorer(); }

  function buildTree(entries) {
    const root = { children: {} };
    entries.forEach((e) => {
      const parts = e.path.split('/'); let n = root;
      parts.forEach((p, i) => { n.children[p] = n.children[p] || { name: p, path: parts.slice(0, i + 1).join('/'), children: {}, entry: null }; n = n.children[p]; });
      n.entry = e;
    });
    return root;
  }
  const ROLE = { conf: 'conf', codes: 'коды', body: 'задание', notes: 'заметки' };
  function renderNode(scope, node, depth) {
    const kids = Object.values(node.children);
    const isDir = (n) => (n.entry ? n.entry.dir : true);
    kids.sort((a, b) => (isDir(b) - isDir(a)) || a.name.localeCompare(b.name, 'ru'));
    return kids.map((n) => {
      const dir = isDir(n), e = n.entry || {}, open = S.exOpen.has(scope + ':' + n.path);
      const key = fileKey(scope, n.path);
      const active = (e.role && scope === 'level' ? e.role : key) === S.file;
      const ico = dir ? (open ? '▾' : '▸') : e.kind === 'image' ? '▣' : /\.md$/i.test(n.name) ? '¶' : '·';
      const row = `<div class="ex-item${dir ? ' dir' : ''}${active ? ' active' : ''}" draggable="true" data-scope="${scope}" data-path="${esc(n.path)}"${dir ? ' data-dir="1"' : ''} data-kind="${esc(e.kind || '')}" data-role="${esc(e.role || '')}" style="padding-left:${6 + depth * 14}px" title="${esc(n.path)}${e.size != null && !dir ? ' · ' + fmtSize(e.size) : ''}">
        <span class="ex-ico">${ico}</span><span class="ex-name">${esc(n.name)}</span>${e.role ? `<span class="ex-role">${ROLE[e.role]}</span>` : ''}<span class="ex-more" data-more title="Действия">⋯</span></div>`;
      return row + (dir && open ? renderNode(scope, n, depth + 1) : '');
    }).join('');
  }
  function renderExplorer() {
    const lv = S.ex.level;
    $('#exLevelTitle').textContent = S.level ? `Уровень ${S.level}${lv && lv.rel ? ' · ' + lv.rel + '/' : ''}` : 'Уровень не выбран';
    $('.ex-sec[data-scope="level"] .ex-btns').hidden = !S.level;
    ['level', 'game'].forEach((scope) => {
      const r = S.ex[scope], box = $('#exTree-' + scope);
      if (scope === 'level' && !S.level) { box.innerHTML = ''; return; }
      box.innerHTML = r && r.entries.length ? renderNode(scope, buildTree(r.entries), 0) : `<div class="muted ex-empty">${scope === 'game' ? 'пусто — общие заметки и картинки игры' : 'пусто'}</div>`;
    });
  }
  function openExFile(scope, path) {
    const e = exEntry(scope, path);
    if (scope === 'level' && e && e.role) { switchFile(e.role); return; }
    const key = fileKey(scope, path);
    if (S.open.some((o) => o.key === key)) { switchFile(key); return; }
    const kind = e ? e.kind : 'text';
    if (kind === 'other') { window.open(fsURL(gameRel(scope, path)), '_blank', 'noopener'); return; }
    (async () => {
      if (kind !== 'image') {
        try { const t = await api('GET', `/api/ui/file?scope=${scope}&n=${S.level || 0}&path=${encodeURIComponent(path)}`); S.raw[key] = t; S.orig[key] = t; }
        catch (err) { toast(String(err.message || err), 'err'); return; }
      }
      S.open.push({ key, scope, path });
      switchFile(key);
    })();
  }
  // Встроенное поле ввода имени (новый файл / папка / переименование).
  function exInline(scope, afterEl, depth, initial, onDone) {
    const box = $('#exTree-' + scope);
    const row = document.createElement('div');
    row.className = 'ex-item ex-edit'; row.style.paddingLeft = (6 + depth * 14) + 'px';
    row.innerHTML = '<input class="mono" spellcheck="false">';
    const inp = row.firstChild; inp.value = initial;
    if (afterEl) afterEl.after(row); else box.prepend(row);
    const dot = initial.lastIndexOf('.');
    inp.focus(); inp.setSelectionRange(0, dot > 0 ? dot : initial.length);
    let done = false;
    const finish = (ok) => { if (done) return; done = true; const v = inp.value.trim(); row.remove(); if (ok && v && v !== initial) onDone(v); else if (!ok || !v) renderExplorer(); };
    inp.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); finish(true); } if (e.key === 'Escape') finish(false); });
    inp.addEventListener('blur', () => finish(true));
  }
  function exRowEl(scope, path) { return $$('#exTree-' + scope + ' .ex-item').find((r) => r.dataset.path === path) || null; }
  function exNew(scope, dir, isDir) {
    if (dir) S.exOpen.add(scope + ':' + dir);
    renderExplorer();
    const parent = dir ? exRowEl(scope, dir) : null;
    exInline(scope, parent, dir ? dir.split('/').length : 0, isDir ? 'папка' : 'заметка.md', async (name) => {
      const path = joinPath(dir, name);
      try {
        if (isDir) await api('POST', '/api/ui/files/mkdir', { scope, n: S.level, path });
        else await api('PUT', `/api/ui/file?scope=${scope}&n=${S.level || 0}&path=${encodeURIComponent(path)}`, '', true);
        await loadExplorer();
        if (!isDir) openExFile(scope, path);
      } catch (e) { toast(String(e.message || e), 'err'); renderExplorer(); }
    });
  }
  async function exRename(scope, path) {
    const el = exRowEl(scope, path); if (!el) return;
    const depth = path.split('/').length - 1;
    el.hidden = true;
    exInline(scope, el, depth, path.split('/').pop(), (name) => exMove(scope, path, joinPath(dirOf(path), name)));
  }
  async function exMove(scope, from, to) {
    if (from === to) return;
    // сервер перепишет ссылки в заметках — сначала сохраняем все правки в них
    await flushAutosave();
    const oldKey = fileKey(scope, from);
    const unsavedMd = Object.keys(S.rawDirty).filter((k) => S.rawDirty[k] && keyKind(k) === 'md');
    if (S.rawDirty[oldKey] || unsavedMd.length) { toast('Сначала сохраните: ' + (unsavedMd.length ? unsavedMd.map(keyName).join(', ') : from), 'err'); return; }
    try {
      const r = await api('POST', '/api/ui/files/rename', { scope, n: S.level, path: from, to });
      // открытые вкладки переезжают вместе с файлом (или содержимым папки)
      S.open.forEach((o) => {
        if (o.scope !== scope || !(o.path === from || o.path.startsWith(from + '/'))) return;
        const nk = fileKey(scope, to + o.path.slice(from.length));
        ['raw', 'orig', 'rawDirty'].forEach((f) => { S[f][nk] = S[f][o.key]; delete S[f][o.key]; });
        dropSession(o.key); MD.forget(o.key);
        if (S.file === o.key) S.file = nk;
        o.key = nk; o.path = to + o.path.slice(from.length);
      });
      await loadExplorer();
      await reloadNotes(r.updated || []);
      if (r.updated && r.updated.length) toast('Ссылки обновлены: ' + r.updated.join(', '), 'ok');
      if (S.tab === 'editor') renderEditor();
    } catch (e) { toast(String(e.message || e), 'err'); renderExplorer(); }
  }
  // reloadNotes перечитывает открытые заметки, которые сервер переписал (пути от папки игры).
  async function reloadNotes(paths) {
    if (!paths.length) return;
    const set = new Set(paths);
    const keys = S.open.filter((o) => set.has(gameRel(o.scope, o.path))).map((o) => o.key);
    if (S.level && S.ex.level && set.has(joinPath(S.ex.level.rel, stdPath('notes').split('/').pop()))) keys.push('notes');
    await Promise.all(keys.map(async (k) => {
      const info = keyInfo(k) || { scope: 'level', path: stdPath('notes').split('/').pop() };
      try {
        const t = await api('GET', `/api/ui/file?scope=${info.scope}&n=${S.level || 0}&path=${encodeURIComponent(info.path)}`);
        S.raw[k] = t; S.rawDirty[k] = false;
        if (k === 'notes') { if (S.data) S.data.raw.notes = t; } else S.orig[k] = t;
        MD.forget(k); dropSession(k);
      } catch (e) { /* файл мог исчезнуть */ }
    }));
    updateDirty();
  }
  async function exDelete(scope, path, isDir) {
    if (!confirm(`Удалить ${isDir ? 'папку' : 'файл'} «${path}»${isDir ? ' со всем содержимым' : ''}?`)) return;
    try {
      await api('DELETE', `/api/ui/file?scope=${scope}&n=${S.level || 0}&path=${encodeURIComponent(path)}`);
      S.open.filter((o) => o.scope === scope && (o.path === path || o.path.startsWith(path + '/'))).forEach((o) => { S.rawDirty[o.key] = false; clearTimeout(MDAUTO.timers[o.key]); delete MDAUTO.timers[o.key]; closeFile(o.key); });
      toast('Удалено: ' + path, 'ok');
      await loadExplorer();
    } catch (e) { toast(String(e.message || e), 'err'); }
  }
  async function exToAssets(scope, path) {
    try {
      const r = await api('POST', '/api/ui/files/to-assets', { scope, n: S.level, path });
      toast(`Скопировано в ассеты: {{${r.name}}} — для игры выполните «Залить ассеты»`, 'ok');
      await loadState();
    } catch (e) { toast(String(e.message || e), 'err'); }
  }
  // exUpload — загрузить файлы в папку dir корня scope; items — File или {file, name}.
  async function exUpload(scope, dir, items) {
    const fd = new FormData();
    fd.append('scope', scope); fd.append('n', String(S.level || 0)); fd.append('dir', dir || '');
    items.forEach((it) => { const f = it.file || it; fd.append('files', f, it.name || f.name); });
    try {
      const r = await fetch('/api/ui/files/upload', { method: 'POST', body: fd });
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || r.statusText);
      if (dir) S.exOpen.add(scope + ':' + dir);
      await loadExplorer();
      toast('Загружено: ' + data.saved.join(', '), 'ok');
      return data.saved;
    } catch (e) { toast(String(e.message || e), 'err'); return []; }
  }
  let exUploadTarget = null;
  $('#exFiles').addEventListener('change', async (e) => {
    const files = Array.from(e.target.files || []); e.target.value = '';
    if (files.length && exUploadTarget) await exUpload(exUploadTarget.scope, exUploadTarget.dir, files);
  });
  function exPickUpload(scope, dir) { exUploadTarget = { scope, dir }; $('#exFiles').click(); }

  function exMenu(row, x, y) {
    const scope = row.dataset.scope, path = row.dataset.path, isDir = !!row.dataset.dir, role = row.dataset.role;
    const items = isDir
      ? [['newfile', 'Новый файл здесь'], ['newdir', 'Новая папка здесь'], ['upload', 'Загрузить сюда'], ['rename', 'Переименовать'], ['delete', 'Удалить']]
      : [['open', 'Открыть'], ['link', 'Вставить ссылку в заметку'], ['toassets', 'В ассеты'], ['raw', 'Открыть в новом окне'], ...(role && role !== 'notes' ? [] : [['rename', 'Переименовать'], ['delete', 'Удалить']])];
    const m = $('#exMenu');
    m.innerHTML = items.map(([a, t]) => `<div data-mact="${a}">${t}</div>`).join('');
    m.hidden = false;
    const host = $('#explorer').getBoundingClientRect();
    m.style.left = Math.max(4, Math.min(x - host.left, host.width - 190)) + 'px';
    m.style.top = (y - host.top + 4) + 'px';
    m.onclick = (e) => {
      const a = e.target.dataset.mact; if (!a) return;
      m.hidden = true;
      if (a === 'newfile' || a === 'newdir') exNew(scope, path, a === 'newdir');
      else if (a === 'upload') exPickUpload(scope, path);
      else if (a === 'rename') exRename(scope, path);
      else if (a === 'delete') exDelete(scope, path, isDir);
      else if (a === 'open') openExFile(scope, path);
      else if (a === 'toassets') exToAssets(scope, path);
      else if (a === 'raw') window.open(fsURL(gameRel(scope, path)), '_blank', 'noopener');
      else if (a === 'link') { if (keyKind(S.file) === 'md') MD.insert(linkFor(scope, path)); else toast('Откройте md-заметку', 'err'); }
    };
  }
  document.addEventListener('mousedown', (e) => { if (!e.target.closest('#exMenu')) $('#exMenu').hidden = true; });

  $$('.ex-sec').forEach((sec) => {
    const scope = sec.dataset.scope;
    sec.querySelector('.ex-btns').addEventListener('click', (e) => {
      const a = e.target.closest('[data-exact]'); if (!a) return;
      if (a.dataset.exact === 'upload') exPickUpload(scope, '');
      else exNew(scope, '', a.dataset.exact === 'newdir');
    });
    const tree = sec.querySelector('.ex-tree');
    tree.addEventListener('click', (e) => {
      const row = e.target.closest('.ex-item'); if (!row || row.classList.contains('ex-edit')) return;
      if (e.target.closest('[data-more]')) { const r = e.target.getBoundingClientRect(); exMenu(row, r.left, r.bottom); return; }
      if (row.dataset.dir) { const k = scope + ':' + row.dataset.path; if (S.exOpen.has(k)) S.exOpen.delete(k); else S.exOpen.add(k); renderExplorer(); return; }
      openExFile(scope, row.dataset.path);
    });
    tree.addEventListener('contextmenu', (e) => { const row = e.target.closest('.ex-item'); if (!row || row.classList.contains('ex-edit')) return; e.preventDefault(); exMenu(row, e.clientX, e.clientY); });
    tree.addEventListener('dragstart', (e) => {
      const row = e.target.closest('.ex-item'); if (!row) return;
      const f = { scope, path: row.dataset.path, dir: !!row.dataset.dir, kind: row.dataset.kind };
      e.dataTransfer.setData('application/x-zp-file', JSON.stringify(f));
      e.dataTransfer.setData('text/plain', f.path);
      e.dataTransfer.effectAllowed = 'copyMove';
    });
    // Сброс на секцию/папку: файлы с диска загружаются, файл из этого же корня переезжает.
    const dropDir = (e) => { const row = e.target.closest('.ex-item'); if (!row) return ''; return row.dataset.dir ? row.dataset.path : dirOf(row.dataset.path); };
    sec.addEventListener('dragover', (e) => {
      const t = e.dataTransfer.types;
      if (!(t.includes('Files') || t.includes('application/x-zp-file'))) return;
      if (scope === 'level' && !S.level) return;
      e.preventDefault(); e.dataTransfer.dropEffect = t.includes('Files') ? 'copy' : 'move';
      sec.classList.add('drop');
      $$('.ex-item.drop', sec).forEach((r) => r.classList.remove('drop'));
      const row = e.target.closest('.ex-item[data-dir]'); if (row) row.classList.add('drop');
    });
    sec.addEventListener('dragleave', (e) => { if (!sec.contains(e.relatedTarget)) { sec.classList.remove('drop'); $$('.ex-item.drop', sec).forEach((r) => r.classList.remove('drop')); } });
    sec.addEventListener('drop', (e) => {
      sec.classList.remove('drop'); $$('.ex-item.drop', sec).forEach((r) => r.classList.remove('drop'));
      const dir = dropDir(e);
      const zf = e.dataTransfer.getData('application/x-zp-file');
      if (zf) {
        e.preventDefault();
        const f = JSON.parse(zf);
        if (f.scope !== scope) { toast('Переносить можно только внутри одной папки (уровень / notes)', 'err'); return; }
        const to = joinPath(dir, f.path.split('/').pop());
        if (to !== f.path && !(f.dir && (dir === f.path || dir.startsWith(f.path + '/')))) exMove(scope, f.path, to);
        return;
      }
      if (e.dataTransfer.files && e.dataTransfer.files.length) { e.preventDefault(); exUpload(scope, dir, [...e.dataTransfer.files]); }
    });
  });
  // Ассеты сайдбара можно тащить в заметку и в task.html.
  $('#assets').addEventListener('dragstart', (e) => {
    const row = e.target.closest('[data-asset]'); if (!row) return;
    e.dataTransfer.setData('application/x-zp-asset', row.dataset.asset);
    e.dataTransfer.setData('text/plain', '{{' + row.dataset.asset + '}}');
    e.dataTransfer.effectAllowed = 'copy';
  });
  // В Ace: ассет → {{имя}}; файл обозревателя — нельзя (на en.cx не заливается).
  E.container.addEventListener('drop', (e) => {
    const dt = e.dataTransfer;
    if (dt.types.includes('application/x-zp-file')) {
      e.preventDefault(); e.stopPropagation();
      toast('Файлы обозревателя не заливаются на en.cx — сначала «⋯ → В ассеты», потом тащите ассет', 'err');
      return;
    }
    const a = dt.getData('application/x-zp-asset');
    if (a) {
      e.preventDefault(); e.stopPropagation();
      const pos = E.renderer.screenToTextCoordinates(e.clientX, e.clientY);
      E.session.insert(pos, '{{' + a + '}}'); E.focus();
    }
  }, true);
  $('#btnExplorer').addEventListener('click', () => {
    const ex = $('#explorer'); ex.hidden = !ex.hidden;
    try { localStorage.setItem('zp.explorer', ex.hidden ? '0' : '1'); } catch (e) { /* ignore */ }
    E.resize(true);
  });
  try { if (localStorage.getItem('zp.explorer') === '0') $('#explorer').hidden = true; } catch (e) { /* ignore */ }

  // conf-форма
  function renderConf() {
    const c = S.conf || {};
    const d = S.data;
    $('#levelTitle').textContent = d ? `Уровень ${d.number}${c.name ? ' «' + c.name + '»' : ''}${d.disabled ? ' · выключен в game.yml' : ''}` : 'Уровень';
    $('#levelPath').textContent = d ? d.files.conf : '';
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
      S.confDirty = false; toast('conf.yml сохранён', 'ok'); await loadState(); await loadLevel(); renderConf(); if (S.tab === 'editor') renderEditor();
    } catch (e) { toast(String(e.message || e), 'err'); }
  });
  $('#btnConfReload').addEventListener('click', async () => { S.confDirty = false; await loadLevel(); renderConf(); });

  // ---------------------------------------------------------------- визуальный редактор
  // task.html показывается в iframe (srcdoc, тот же origin) с CSS движка и <style> уровня;
  // <script>/<style>/комментарии вырезаются в защищённые блоки-плейсхолдеры и при
  // сохранении возвращаются на место как есть. {{ассет}} ↔ /assets/ассет.
  const VE = { frame: $('#visualFrame'), doc: null, root: null, blocks: [], loadedSrc: null, dirty: false, timer: null, range: null, pop: null };
  const veAssetsFwd = (s) => s.replace(/\{\{\s*([^{}]+?)\s*\}\}/g, (m, n) => '/assets/' + encodeURIComponent(n));
  const veAssetsBack = (s) => s.replace(/\/assets\/([^"'()\s<>]+)/g, (m, n) => { try { return '{{' + decodeURIComponent(n) + '}}'; } catch (e) { return m; } });
  const veProtectRe = /<!--[\s\S]*?-->|<script\b[\s\S]*?<\/script\s*>|<style\b[\s\S]*?<\/style\s*>/gi;

  function veProtect(html) {
    const blocks = [];
    const out = html.replace(veProtectRe, (m) => {
      const kind = m.startsWith('<!--') ? 'комментарий' : /^<script/i.test(m) ? 'script' : 'style';
      const i = blocks.push({ kind, text: m }) - 1;
      return `<span class="ve-block" contenteditable="false" data-ve="${i}" title="${kind}: сохраняется как есть; удалите блок — удалится код">⚙ ${kind} · ${m.split('\n').length} стр.</span>`;
    });
    return { html: out, blocks };
  }

  function renderVisual() {
    const d = S.data;
    $('#visualTitle').textContent = 'Визуальный · уровень ' + (S.level || '—');
    $('#visualPath').textContent = d ? (d.files.body || d.files.dir + '/task.html') : '';
    if (!S.level) { VE.frame.srcdoc = '<p style="font-family:sans-serif;color:#888;padding:20px">Выберите уровень.</p>'; VE.root = null; return; }
    const src = S.raw.body || '';
    if (VE.loadedSrc === src && VE.root) return;
    VE.loadedSrc = src; VE.dirty = false;
    const { html, blocks } = veProtect(src);
    VE.blocks = blocks;
    const kinds = blocks.map((b) => b.kind);
    $('#veBlocks').textContent = blocks.length ? '⚙ блоков: ' + blocks.length + ' (' + Array.from(new Set(kinds)).join(', ') + ')' : '';
    const st = S.state, eb = st.engineBase, ver = st.engineVer;
    const styles = blocks.filter((b) => b.kind === 'style').map((b) => veAssetsFwd(b.text)).join('\n');
    VE.frame.srcdoc = `<!doctype html><html lang="ru"><head><meta charset="utf-8">
<link href="${eb}/css/v2/en/engines/engine.css?ver=${ver}" rel="stylesheet">
<link href="${eb}/css/v2/en/engines/real.css?ver=${ver}" rel="stylesheet">
<link href="${eb}/css/v2/en/engines/engine_adaptive.css?ver=${ver}" rel="stylesheet">
<style>html,body{min-width:0;margin:0;min-height:100%}.container{padding:0}.content{margin-left:0;padding:12px 16px}#ve{outline:none;min-height:70vh}#ve:empty::before{content:"Начните печатать или вставьте текст…";color:#777}
.ve-block{display:inline-block;padding:1px 8px;margin:2px 0;border:1px dashed #e0c97f;border-radius:6px;background:rgba(224,201,127,.14);color:#e0c97f;font:12px/1.5 monospace;cursor:default;user-select:none}</style>
${styles}
</head><body><div class="container"><div class="content"><div class="task"><div id="ve" contenteditable="true">${veAssetsFwd(html)}</div></div></div></div></body></html>`;
  }
  VE.frame.addEventListener('load', () => {
    VE.doc = VE.frame.contentDocument; VE.root = VE.doc && VE.doc.getElementById('ve');
    if (!VE.root) return;
    VE.root.addEventListener('input', () => { VE.dirty = true; clearTimeout(VE.timer); VE.timer = setTimeout(veFlush, 250); });
    VE.root.addEventListener('paste', (e) => {
      const t = (e.clipboardData || window.clipboardData).getData('text/plain'); if (!t) return;
      e.preventDefault(); VE.doc.execCommand('insertText', false, t);
    });
    VE.doc.addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); veSave(); } });
    VE.doc.addEventListener('selectionchange', () => { const s = VE.doc.getSelection(); if (s && s.rangeCount && VE.root.contains(s.anchorNode)) { VE.range = s.getRangeAt(0).cloneRange(); veSyncBlockSelect(); } });
  });
  function veSerialize() {
    if (!VE.root) return S.raw.body || '';
    let html = VE.root.innerHTML;
    html = html.replace(/<span[^>]*\bdata-ve="(\d+)"[^>]*>[\s\S]*?<\/span>/g, (m, i) => (VE.blocks[+i] ? VE.blocks[+i].text : ''));
    return veAssetsBack(html);
  }
  function veFlush() {
    // Только после реальных правок: браузер нормализует HTML, и без этого флага
    // простое открытие вкладки помечало бы файл изменённым.
    if (!VE.dirty || !VE.root || !S.level) return;
    const text = veSerialize();
    if (text === VE.loadedSrc) return;
    S.raw.body = text; VE.loadedSrc = text;
    S.rawDirty.body = text !== (S.data && S.data.raw ? S.data.raw.body || '' : '');
    updateDirty();
  }
  async function veSave() { clearTimeout(VE.timer); veFlush(); if (await saveFile('body', S.raw.body || '')) VE.loadedSrc = S.raw.body; }
  $('#btnVisualSave').addEventListener('click', veSave);
  function veRestore() { if (!VE.doc) return; VE.frame.contentWindow.focus(); const s = VE.doc.getSelection(); if (VE.range && s) { s.removeAllRanges(); s.addRange(VE.range); } }
  function veExec(cmd, val) { if (!VE.doc) return; veRestore(); VE.doc.execCommand(cmd, false, val); VE.root.dispatchEvent(new Event('input')); }
  $$('.ve-tools [data-cmd]').forEach((b) => { b.addEventListener('mousedown', (e) => e.preventDefault()); b.addEventListener('click', () => veExec(b.dataset.cmd)); });
  $('#veBlock').addEventListener('change', (e) => veExec('formatBlock', '<' + e.target.value + '>'));
  function veSyncBlockSelect() {
    let n = VE.doc.getSelection().anchorNode; if (!n) return;
    if (n.nodeType === 3) n = n.parentNode;
    while (n && n !== VE.root && !/^(P|H[1-6]|BLOCKQUOTE|PRE|DIV)$/.test(n.tagName)) n = n.parentNode;
    const tag = n && n !== VE.root ? n.tagName.toLowerCase() : 'p';
    const sel = $('#veBlock'); if (Array.from(sel.options).some((o) => o.value === tag)) sel.value = tag;
  }
  function vePopOpen(kind) {
    VE.pop = kind;
    const pop = $('#vePop'); pop.hidden = false;
    $('#vePopTitle').textContent = kind === 'link' ? 'Ссылка' : 'Картинка';
    const sel = $('#vePopAsset');
    sel.hidden = false;
    sel.innerHTML = '<option value="">— ассет —</option>' + S.state.assets.map((a) => `<option value="{{${esc(a.name)}}}">{{${esc(a.name)}}}</option>`).join('');
    $('#vePopUrl').value = ''; $('#vePopUrl').placeholder = kind === 'link' ? 'https://… или {{файл}}' : 'https://…/картинка.jpg или {{картинка.jpg}}';
    $('#vePopUrl').focus();
  }
  $('#veLink').addEventListener('mousedown', (e) => e.preventDefault()); $('#veLink').addEventListener('click', () => vePopOpen('link'));
  $('#veImage').addEventListener('mousedown', (e) => e.preventDefault()); $('#veImage').addEventListener('click', () => vePopOpen('image'));
  $('#vePopAsset').addEventListener('change', (e) => { if (e.target.value) $('#vePopUrl').value = e.target.value; });
  $('#vePopCancel').addEventListener('click', () => { $('#vePop').hidden = true; veRestore(); });
  $('#vePopUrl').addEventListener('keydown', (e) => { if (e.key === 'Enter') $('#vePopOk').click(); if (e.key === 'Escape') $('#vePopCancel').click(); });
  $('#vePopOk').addEventListener('click', () => {
    const url = $('#vePopUrl').value.trim(); if (!url) return;
    $('#vePop').hidden = true;
    const href = veAssetsFwd(url);
    if (VE.pop === 'link') veExec('createLink', href);
    else veExec('insertHTML', `<img src="${esc(href)}" alt="">`);
  });

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
    setTab(S.state && !S.state.game ? 'commands' : (location.hash || '#commands').slice(1));
    // если задание уже идёт (страницу перезагрузили) — подхватить лог
    if (S.state && S.state.job && !S.state.job.done) { S.job = S.state.job.id; S.jobFrom = 0; $('#log').textContent = ''; pollJob(); }
    setInterval(async () => { if (S.tab === 'commands' && !S.job) { try { await loadState(); } catch (e) { /* ignore */ } } }, 15000);
  })();
})();
