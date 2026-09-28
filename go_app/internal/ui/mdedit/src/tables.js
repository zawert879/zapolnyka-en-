// Таблицы GFM в Live Preview: пока курсор вне таблицы (или редактор не в фокусе),
// таблица целиком заменяется отрисованным виджетом; клик по ней открывает исходник.
// Замена через несколько строк возможна только из StateField, поэтому это отдельно
// от ViewPlugin в livepreview.js.
import { Decoration, WidgetType, EditorView } from '@codemirror/view';
import { StateField, StateEffect } from '@codemirror/state';
import { syntaxTree } from '@codemirror/language';

const focusEffect = StateEffect.define();
const focusField = StateField.define({
  create: () => false,
  update(v, tr) { for (const e of tr.effects) if (e.is(focusEffect)) v = e.value; return v; },
});

const escHTML = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

// splitRow — ячейки строки таблицы: «|» внутри `кода`, [[вики|подписи]] и экранированный «\|» не делят.
export function splitRow(line) {
  let s = line.trim();
  if (s.startsWith('|')) s = s.slice(1);
  if (s.endsWith('|') && !s.endsWith('\\|')) s = s.slice(0, -1);
  const cells = [];
  let cur = '', code = false, wiki = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '\\' && s[i + 1] === '|') { cur += '|'; i++; continue; }
    if (c === '`') code = !code;
    if (!code && c === '[' && s[i + 1] === '[') wiki++;
    if (!code && c === ']' && s[i + 1] === ']' && wiki) wiki--;
    if (c === '|' && !code && !wiki) { cells.push(cur.trim()); cur = ''; continue; }
    cur += c;
  }
  cells.push(cur.trim());
  return cells;
}

function alignOf(cell) {
  const l = cell.startsWith(':'), r = cell.endsWith(':');
  return l && r ? 'center' : r ? 'right' : l ? 'left' : '';
}

// inlineHTML — упрощённый рендер markdown внутри ячейки.
function inlineHTML(md, resolve) {
  const codes = [];
  let s = md.replace(/(`+)([\s\S]*?)\1/g, (m, t, c) => '\u0000' + (codes.push(c) - 1) + '\u0000');
  s = escHTML(s);
  const attr = (v) => escHTML(v);
  s = s.replace(/!\[\[([^\]|]+)(?:\|(\d+))?\]\]/g, (m, t, w) => `<img class="cm-lp-cellimg" src="${attr(resolve(t.trim(), true) || '')}"${w ? ` style="width:${w}px"` : ''} alt="">`);
  s = s.replace(/!\[([^\]]*)\]\(([^)\s]+)\)/g, (m, a, u) => `<img class="cm-lp-cellimg" src="${attr(resolve(u.replace(/&amp;/g, '&'), false) || '')}" alt="${a}">`);
  s = s.replace(/\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g, (m, t, a) => `<span class="cm-lp-link cm-lp-wiki" data-href="${attr(t.trim())}" data-wiki="1">${a || t}</span>`);
  s = s.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (m, t, u) => `<span class="cm-lp-link" data-href="${u}">${t}</span>`);
  s = s.replace(/\{\{\s*([^{}]+?)\s*\}\}/g, (m) => `<span class="cm-lp-asset">${m}</span>`);
  s = s.replace(/(\*\*|__)(?=\S)([\s\S]*?\S)\1/g, '<strong>$2</strong>');
  s = s.replace(/(\*|_)(?=\S)([\s\S]*?\S)\1/g, '<em>$2</em>');
  s = s.replace(/~~(?=\S)([\s\S]*?\S)~~/g, '<s>$1</s>');
  s = s.replace(/<br\s*\/?>/gi, '<br>');
  return s.replace(/\u0000(\d+)\u0000/g, (m, i) => `<code class="cm-lp-code">${escHTML(codes[+i])}</code>`);
}

class TableWidget extends WidgetType {
  constructor(text, opts) { super(); this.text = text; this.opts = opts; }
  eq(o) { return o.text === this.text; }
  toDOM(view) {
    const lines = this.text.split('\n').filter((l) => l.trim());
    const head = splitRow(lines[0] || ''), aligns = splitRow(lines[1] || '').map(alignOf);
    const cell = (tag, c, i) => `<${tag}${aligns[i] ? ` style="text-align:${aligns[i]}"` : ''}>${inlineHTML(c, this.opts.resolve)}</${tag}>`;
    const body = lines.slice(2).map((l) => {
      const cells = splitRow(l);
      while (cells.length < head.length) cells.push('');
      return '<tr>' + cells.slice(0, head.length).map((c, i) => cell('td', c, i)).join('') + '</tr>';
    }).join('');
    const wrap = document.createElement('div');
    wrap.className = 'cm-lp-tablewrap';
    wrap.innerHTML = `<table class="cm-lp-tbl"><thead><tr>${head.map((c, i) => cell('th', c, i)).join('')}</tr></thead><tbody>${body}</tbody></table>`;
    wrap.addEventListener('mousedown', (e) => {
      if (e.button !== 0) return;
      e.preventDefault();
      const link = e.target.closest('[data-href]');
      if (link) { this.opts.onOpenLink(link.dataset.href, !!link.dataset.wiki); return; }
      // клик по таблице — курсор в её начало, таблица показывается исходником
      const pos = view.posAtDOM(wrap);
      view.dispatch({ selection: { anchor: pos } });
      view.focus();
    });
    return wrap;
  }
  ignoreEvent() { return true; }
}

function build(state, opts) {
  const focused = state.field(focusField);
  const sel = state.selection.ranges;
  const out = [];
  syntaxTree(state).iterate({
    enter(n) {
      if (n.name !== 'Table') return;
      const from = state.doc.lineAt(n.from).from, to = state.doc.lineAt(n.to).to;
      const touched = focused && sel.some((r) => r.to >= from && r.from <= to);
      if (!touched) out.push(Decoration.replace({ widget: new TableWidget(state.doc.sliceString(from, to), opts), block: true }).range(from, to));
      return false;
    },
  });
  return Decoration.set(out, true);
}

export function tables(opts) {
  const field = StateField.define({
    create: (state) => build(state, opts),
    update(deco, tr) {
      if (tr.docChanged || tr.selection || tr.effects.some((e) => e.is(focusEffect)) || syntaxTree(tr.startState) !== syntaxTree(tr.state)) return build(tr.state, opts);
      return deco;
    },
    provide: (f) => EditorView.decorations.from(f),
  });
  const focus = EditorView.domEventHandlers({
    focus(e, view) { if (!view.state.field(focusField)) view.dispatch({ effects: focusEffect.of(true) }); },
    blur(e, view) { if (view.state.field(focusField)) setTimeout(() => { try { if (!view.hasFocus) view.dispatch({ effects: focusEffect.of(false) }); } catch (err) { /* редактор уже закрыт */ } }, 0); },
  });
  return [focusField, field, focus];
}
