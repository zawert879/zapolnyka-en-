// ════════════════════════════════════════════════════
//   НАСТРОЙКИ — заполни перед запуском
// ════════════════════════════════════════════════════

const THRESHOLD = 15;       // очков нужно набрать
const CODE = 'ОТВЕТ123';    // код закрытия задания

// Секрет: оставь пустым — сгенерируется сам.
// После первого запуска скопируй сгенерированный секрет сюда,
// чтобы при перегенерации подписи бонусов не менялись.
const SECRET = '';

// Типы бонусов — id уникальный, value — очков за открытие
const BONUS_TYPES: Array<{ id: string; value: number; name: string }> = [
  { id: 'mon', value: 1, name: 'Монета'   },
  { id: 'kam', value: 3, name: 'Камень'   },
  { id: 'sam', value: 5, name: 'Самоцвет' },
];

// ════════════════════════════════════════════════════
//   ГЕНЕРАТОР — ниже не трогай
// ════════════════════════════════════════════════════

function genSecret(): string {
  let s = '';
  while (s.length < 16) s += Math.random().toString(36).slice(2);
  return s.slice(0, 16);
}

function hash(n: number, id: string, secret: string): string {
  let h = 0x811c;
  const s = String(n) + id + secret;
  for (let i = 0; i < s.length; i++) h = ((h ^ s.charCodeAt(i)) * 0x1b3) >>> 0;
  return h.toString(36);
}

function encrypt(code: string, secret: string): string {
  const enc = encodeURIComponent(code);
  let bin = '';
  for (let i = 0; i < enc.length; i++)
    bin += String.fromCharCode(enc.charCodeAt(i) ^ secret.charCodeAt(i % secret.length));
  return btoa(bin);
}

function plural(n: number): string {
  if (n === 1) return 'очко';
  if (n >= 2 && n <= 4) return 'очка';
  return 'очков';
}

function buildBody(secret: string): string {
  const enc = encrypt(CODE, secret);

  const css =
    '#_pb{position:fixed;top:0;left:0;right:0;z-index:9999;' +
    'background:rgba(10,14,20,.97);backdrop-filter:blur(6px);' +
    'padding:10px 24px 8px;box-shadow:0 2px 28px rgba(0,0,0,.8);' +
    'font-family:"Courier New",monospace;border-bottom:1px solid #1e3a5f}' +
    '#_pb-lbl{color:#3d6b9e;font-size:10px;letter-spacing:3px;text-transform:uppercase;margin-bottom:6px}' +
    '#_pb-track{background:#0a0f18;border-radius:12px;height:16px;' +
    'overflow:hidden;border:1px solid #1d2d44}' +
    '#_pb-fill{height:100%;width:0%;border-radius:12px;' +
    'background:linear-gradient(90deg,#0369a1 0%,#0ea5e9 60%,#7dd3fc 100%);' +
    'transition:width .65s cubic-bezier(.4,0,.2,1);' +
    'box-shadow:0 0 14px rgba(14,165,233,.45)}' +
    '#_pb-pts{color:#38bdf8;font-size:11px;text-align:right;margin-top:4px}' +
    '#_pb-clbl{display:none;color:#6b7280;font-size:10px;letter-spacing:3px;' +
    'text-align:center;text-transform:uppercase;margin-bottom:2px}' +
    '#_pb-code{display:none;color:#f59e0b;font-size:28px;font-weight:700;' +
    'text-align:center;letter-spacing:10px;padding:4px 0 2px;' +
    'text-shadow:0 0 24px rgba(245,158,11,.65);animation:_pg 1.4s ease-in-out infinite}' +
    '@keyframes _pg{0%,100%{opacity:1}50%{opacity:.6}}';

  // localStorage не используется — очки считаются заново при каждой загрузке
  // из help-скриптов уже открытых бонусов, которые en.cx вызывает сам
  const js =
    '(function(){' +
    'var _s="' + secret + '",_e="' + enc + '",_t=' + THRESHOLD + ',_u={},_p=0;' +
    'function _h(n,id){var h=0x811c,s=String(n)+id+_s;' +
    'for(var i=0;i<s.length;i++)h=(h^s.charCodeAt(i))*0x1b3>>>0;return h.toString(36);}' +
    'function _d(){return decodeURIComponent([...atob(_e)]' +
    '.map(function(c,i){return String.fromCharCode(c.charCodeAt(0)^_s.charCodeAt(i%_s.length));}).join(""));}' +
    'function _ui(p){' +
    'document.getElementById("_pb-fill").style.width=(Math.min(p,_t)/_t*100)+"%";' +
    'if(p>=_t){' +
    'document.getElementById("_pb-pts").style.display="none";' +
    'document.getElementById("_pb-track").style.display="none";' +
    'document.getElementById("_pb-lbl").style.display="none";' +
    'document.getElementById("_pb-clbl").style.display="block";' +
    'var c=document.getElementById("_pb-code");c.textContent=_d();c.style.display="block";' +
    '}else{document.getElementById("_pb-pts").textContent=p+" / "+_t+" очков";}}' +
    'window.addPoints=function(n,id,sig){' +
    'if(_h(n,id)!==sig)return;' +
    'if(_u[id])return;' +
    '_u[id]=1;_p+=n;_ui(_p);};' +
    '})();';

  return (
    '<style>' + css + '</style>\n' +
    '<div id="_pb">\n' +
    '  <div id="_pb-lbl">ШКАЛА ПРОГРЕССА</div>\n' +
    '  <div id="_pb-track"><div id="_pb-fill"></div></div>\n' +
    '  <div id="_pb-pts">0 / ' + THRESHOLD + ' очков</div>\n' +
    '  <div id="_pb-clbl">КОД ЗАКРЫТИЯ</div>\n' +
    '  <div id="_pb-code"></div>\n' +
    '</div>\n' +
    '<script>' + js + '<\/script>'
  );
}

function buildHelp(value: number, id: string, secret: string): string {
  const sig = hash(value, id, secret);
  return (
    '+' + value + ' ' + plural(value) +
    '<script>(function(){addPoints(' + value + ',"' + id + '","' + sig + '")})()</script>'
  );
}

// ── Запуск ──

const secret = SECRET || genSecret();
const SEP = '═'.repeat(68);

console.log('\n' + SEP);
console.log('  ГЕНЕРАТОР: ШКАЛА ПРОГРЕССА');
console.log(SEP);
console.log('  Порог:        ' + THRESHOLD + ' очков');
console.log('  Код закрытия: ' + CODE);
console.log('  Секрет:       ' + secret + '  ← сохрани в SECRET если нужна перегенерация');
console.log(SEP);

console.log('\n\n' + SEP);
console.log('  ТЕЛО ЗАДАНИЯ');
console.log(SEP + '\n');
console.log(buildBody(secret));
console.log('\n' + SEP);

console.log('\n\n' + SEP);
console.log('  БОНУСЫ  (поле Help для каждого типа)');
console.log(SEP);

for (const b of BONUS_TYPES) {
  console.log('\n  ' + b.name + ' (' + b.value + ' ' + plural(b.value) + ')');
  console.log('  Help: ' + buildHelp(b.value, b.id, secret));
}

console.log('\n' + SEP + '\n');
