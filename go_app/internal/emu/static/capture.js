// capture.js — снятие реальной play-страницы en.cx в локальный эмулятор.
//
// Выполнить в консоли браузера (или через Chrome MCP) на странице
// https://<домен>/gameengines/encounter/play/<gid>/ при запущенном эмуляторе:
//
//   EMU_CAPTURE('L06-before')           // имя снимка: буквы, цифры, _ . -
//   EMU_CAPTURE('L06-mid', 8090)        // порт эмулятора, если не 8090
//
// Страница сама делает fetch ?json=1 и POST'ит свой outerHTML + JSON на
// http://127.0.0.1:<port>/api/snapshot → data/<игра>/snapshots/<имя>.html|.json.
// Помощники для прохождения уровня: EMU_SEND('код') — отправить ответ,
// EMU_PENALTY(helpId) — взять штрафную подсказку (после — location.reload()).
(function () {
  function port(p) { return p || 8090; }
  window.EMU_CAPTURE = async function (name, p) {
    var json = '';
    try {
      json = await fetch(location.pathname + '?json=1&lang=ru', { credentials: 'include' }).then(function (r) { return r.text(); });
    } catch (e) { console.warn('json fetch failed', e); }
    var html = '<!DOCTYPE html>\n' + document.documentElement.outerHTML;
    var res = await fetch('http://127.0.0.1:' + port(p) + '/api/snapshot', {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body: JSON.stringify({ name: name, html: html, json: json, url: location.href })
    }).then(function (r) { return r.json(); });
    console.log('EMU_CAPTURE', name, res);
    return res;
  };
  window.EMU_SEND = async function (code) {
    var lid = document.querySelector('input[name=LevelId]').value;
    var lnum = document.querySelector('input[name=LevelNumber]').value;
    var body = new URLSearchParams({ LevelId: lid, LevelNumber: lnum, 'LevelAction.Answer': code });
    var r = await fetch(location.pathname, { method: 'POST', body: body, credentials: 'include' });
    console.log('EMU_SEND', code, r.status);
    return r.status;
  };
  window.EMU_PENALTY = async function (helpId, pact) {
    var r = await fetch(location.pathname + '?pid=' + helpId + '&pact=' + (pact || 1), { credentials: 'include' });
    console.log('EMU_PENALTY', helpId, r.status);
    return r.status;
  };
  console.log('capture.js ready: EMU_CAPTURE(name), EMU_SEND(code), EMU_PENALTY(helpId)');
})();
