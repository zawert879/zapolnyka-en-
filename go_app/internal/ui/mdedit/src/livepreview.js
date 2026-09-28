// Live Preview в духе Obsidian: вне строк с курсором markdown-разметка прячется,
// картинки и ссылки рендерятся; на строке с курсором виден исходник.
import { Decoration, ViewPlugin, WidgetType, EditorView } from '@codemirror/view';
import { syntaxTree } from '@codemirror/language';

// ---------------------------------------------------------------- [[вики-ссылки]]

// Расширение парсера @lezer/markdown: [[цель|подпись]] и ![[файл|ширина]].
export const wikiLinks = {
  defineNodes: ['WikiLink', 'WikiEmbed'],
  parseInline: [{
    name: 'WikiLink',
    before: 'Link',
    parse(cx, next, pos) {
      let start = pos, embed = false;
      if (next === 33 /* ! */ && cx.char(pos + 1) === 91 && cx.char(pos + 2) === 91) { embed = true; pos += 1; } else if (next !== 91 || cx.char(pos + 1) !== 91) return -1;
      const text = cx.slice(pos + 2, Math.min(cx.end, pos + 2 + 500));
      const close = text.indexOf(']]');
      if (close <= 0 || text.slice(0, close).includes('\n') || text.slice(0, close).includes('[')) return -1;
      const end = pos + 2 + close + 2;
      return cx.addElement(cx.elt(embed ? 'WikiEmbed' : 'WikiLink', start, end));
    },
  }],
};

// ---------------------------------------------------------------- виджеты

const IMG_RE = /\.(png|jpe?g|gif|webp|svg|bmp|ico|avif)$/i;

class ImageWidget extends WidgetType {
  constructor(src, alt, width, raw) {
    super(); this.src = src; this.alt = alt; this.width = width;
    try { this.raw = decodeURIComponent(raw); } catch (e) { this.raw = raw; }
  }
  eq(o) { return o.src === this.src && o.alt === this.alt && o.width === this.width; }
  toDOM() {
    const wrap = document.createElement('span');
    wrap.className = 'cm-lp-imgwrap';
    if (!this.src) {
      wrap.className = 'cm-lp-broken';
      wrap.textContent = '⚠ не найдено: ' + this.raw;
      return wrap;
    }
    const img = document.createElement('img');
    img.className = 'cm-lp-img';
    img.src = this.src;
    img.alt = this.alt || '';
    img.title = this.raw;
    img.draggable = false;
    if (this.width) img.style.width = this.width + 'px';
    img.onerror = () => { wrap.className = 'cm-lp-broken'; wrap.textContent = '⚠ не найдено: ' + this.raw; };
    wrap.appendChild(img);
    return wrap;
  }
  ignoreEvent() { return false; }
}

class LinkWidget extends WidgetType {
  constructor(text, href, wiki) { super(); this.text = text; this.href = href; this.wiki = wiki; }
  eq(o) { return o.text === this.text && o.href === this.href; }
  toDOM() {
    const a = document.createElement('span');
    a.className = 'cm-lp-link' + (this.wiki ? ' cm-lp-wiki' : '');
    a.dataset.href = this.href;
    if (this.wiki) a.dataset.wiki = '1';
    a.textContent = this.text;
    return a;
  }
  ignoreEvent() { return false; }
}

class BulletWidget extends WidgetType {
  eq() { return true; }
  toDOM() { const s = document.createElement('span'); s.className = 'cm-lp-bullet'; s.textContent = '•'; return s; }
}

class CheckboxWidget extends WidgetType {
  constructor(checked, pos) { super(); this.checked = checked; this.pos = pos; }
  eq(o) { return o.checked === this.checked && o.pos === this.pos; }
  toDOM(view) {
    const box = document.createElement('input');
    box.type = 'checkbox';
    box.className = 'cm-lp-task';
    box.checked = this.checked;
    box.addEventListener('mousedown', (e) => {
      e.preventDefault();
      view.dispatch({ changes: { from: this.pos + 1, to: this.pos + 2, insert: this.checked ? ' ' : 'x' } });
    });
    return box;
  }
  ignoreEvent() { return true; }
}

class HrWidget extends WidgetType {
  eq() { return true; }
  toDOM() { const s = document.createElement('span'); s.className = 'cm-lp-hr'; return s; }
}

// ---------------------------------------------------------------- разбор

// parseWiki — «цель|подпись» → {target, label, width}.
function parseWiki(inner) {
  const bar = inner.indexOf('|');
  const target = (bar < 0 ? inner : inner.slice(0, bar)).replace(/\\$/, '').trim(); // «\|» в таблицах
  const label = bar < 0 ? '' : inner.slice(bar + 1).trim();
  const width = /^\d+$/.test(label) ? +label : 0;
  return { target, label: width ? '' : label, width };
}

// Картинка ![alt|300](url "title"): alt, ширина и адрес из исходника узла.
function parseImage(src) {
  const m = /^!\[([^\]]*)\]\(\s*(<[^>]*>|[^\s)]*)/.exec(src);
  if (!m) return null;
  let alt = m[1], width = 0;
  const w = /\|(\d+)$/.exec(alt);
  if (w) { width = +w[1]; alt = alt.slice(0, w.index); }
  let url = m[2];
  if (url.startsWith('<')) url = url.slice(1, -1);
  return { alt, width, url };
}

function activeLines(view) {
  const lines = new Set();
  if (!view.hasFocus) return lines;
  for (const r of view.state.selection.ranges) {
    const a = view.state.doc.lineAt(r.from).number, b = view.state.doc.lineAt(r.to).number;
    for (let i = a; i <= b; i++) lines.add(i);
  }
  return lines;
}

const hide = Decoration.replace({});
const mk = (cls) => Decoration.mark({ class: cls });
const line = (cls) => Decoration.line({ class: cls });

function build(view, opts) {
  const { state } = view;
  const doc = state.doc;
  const act = activeLines(view);
  const out = [];
  const isActive = (from, to = from) => {
    const a = doc.lineAt(from).number, b = doc.lineAt(to).number;
    for (let i = a; i <= b; i++) if (act.has(i)) return true;
    return false;
  };
  const sameLine = (from, to) => doc.lineAt(from).number === doc.lineAt(to).number;
  const add = (from, to, deco) => out.push(deco.range(from, to));

  for (const { from, to } of view.visibleRanges) {
    syntaxTree(state).iterate({
      from, to,
      enter(node) {
        const name = node.name;
        const nf = node.from, nt = node.to;
        let m;
        if ((m = /^ATXHeading(\d)$/.exec(name))) {
          add(nf, nf, line('cm-lp-h' + m[1]));
          if (!isActive(nf)) {
            const mark = node.node.firstChild;
            if (mark && mark.name === 'HeaderMark') {
              const end = Math.min(nt, mark.to + (doc.sliceString(mark.to, mark.to + 1) === ' ' ? 1 : 0));
              add(mark.from, end, hide);
            }
          }
          return;
        }
        switch (name) {
          case 'Emphasis': add(nf, nt, mk('cm-lp-em')); break;
          case 'StrongEmphasis': add(nf, nt, mk('cm-lp-strong')); break;
          case 'Strikethrough': add(nf, nt, mk('cm-lp-strike')); break;
          case 'InlineCode': add(nf, nt, mk('cm-lp-code')); break;
          case 'EmphasisMark': case 'StrikethroughMark':
            if (!isActive(nf)) add(nf, nt, hide);
            break;
          case 'CodeMark':
            if (node.node.parent && node.node.parent.name === 'InlineCode' && !isActive(nf)) add(nf, nt, hide);
            break;
          case 'Blockquote': {
            for (let p = nf; p <= nt;) { const l = doc.lineAt(p); add(l.from, l.from, line('cm-lp-quote')); p = l.to + 1; }
            break;
          }
          case 'QuoteMark':
            if (!isActive(nf)) add(nf, Math.min(doc.lineAt(nf).to, nt + (doc.sliceString(nt, nt + 1) === ' ' ? 1 : 0)), hide);
            break;
          case 'FencedCode': case 'CodeBlock': {
            const fenced = name === 'FencedCode';
            const first = doc.lineAt(nf).number, last = doc.lineAt(nt).number;
            const act2 = isActive(nf, nt);
            for (let i = first; i <= last; i++) {
              const l = doc.line(i);
              const fence = fenced && (i === first || (i === last && /^\s*(```|~~~)/.test(l.text)));
              add(l.from, l.from, line('cm-lp-codeblock' + (i === first ? ' cm-lp-cb-first' : '') + (i === last ? ' cm-lp-cb-last' : '') + (fence && !act2 ? ' cm-lp-fence' : '')));
            }
            return false;
          }
          case 'Table': {
            for (let p = nf; p <= nt;) { const l = doc.lineAt(p); add(l.from, l.from, line('cm-lp-table')); p = l.to + 1; }
            return false;
          }
          case 'HorizontalRule':
            if (!isActive(nf)) add(nf, nt, Decoration.replace({ widget: new HrWidget() }));
            break;
          case 'ListMark': {
            const list = node.node.parent && node.node.parent.parent;
            const task = node.node.nextSibling && node.node.nextSibling.name === 'Task';
            if (list && list.name === 'BulletList' && !task && !isActive(nf)) add(nf, nt, Decoration.replace({ widget: new BulletWidget() }));
            if (task && !isActive(nf)) add(nf, Math.min(nt + 1, doc.lineAt(nf).to), hide);
            break;
          }
          case 'TaskMarker': {
            const checked = /x/i.test(doc.sliceString(nf, nt));
            if (checked) add(doc.lineAt(nf).from, doc.lineAt(nf).from, line('cm-lp-done'));
            if (!isActive(nf)) add(nf, Math.min(nt + (doc.sliceString(nt, nt + 1) === ' ' ? 1 : 0), doc.lineAt(nf).to), Decoration.replace({ widget: new CheckboxWidget(checked, nf) }));
            break;
          }
          case 'Link': {
            if (!sameLine(nf, nt)) return false;
            const marks = [];
            let url = '';
            for (let c = node.node.firstChild; c; c = c.nextSibling) {
              if (c.name === 'LinkMark') marks.push(c);
              if (c.name === 'URL') url = doc.sliceString(c.from, c.to);
            }
            if (marks.length < 2) return false;
            const textFrom = marks[0].to, textTo = marks[1].from;
            add(textFrom, textTo, Decoration.mark({ class: 'cm-lp-link', attributes: { 'data-href': url.replace(/^<|>$/g, '') } }));
            if (!isActive(nf)) { add(nf, textFrom, hide); add(textTo, nt, hide); }
            return false;
          }
          case 'URL':
            if (node.node.parent && node.node.parent.name === 'Autolink') break;
            if (!node.node.parent || node.node.parent.name === 'Document' || node.node.parent.name === 'Paragraph') {
              add(nf, nt, Decoration.mark({ class: 'cm-lp-link', attributes: { 'data-href': doc.sliceString(nf, nt) } }));
            }
            break;
          case 'Image': {
            if (!sameLine(nf, nt)) return false;
            const img = parseImage(doc.sliceString(nf, nt));
            if (!img) return false;
            const w = new ImageWidget(opts.resolve(img.url, false), img.alt, img.width, img.url);
            if (isActive(nf)) add(nt, nt, Decoration.widget({ widget: w, side: 1 }));
            else add(nf, nt, Decoration.replace({ widget: w }));
            return false;
          }
          case 'WikiEmbed': case 'WikiLink': {
            const embed = name === 'WikiEmbed';
            const inner = doc.sliceString(nf + (embed ? 3 : 2), nt - 2);
            const { target, label, width } = parseWiki(inner);
            const active = isActive(nf);
            if (embed && IMG_RE.test(target.split('#')[0])) {
              const w = new ImageWidget(opts.resolve(target, true), label, width, target);
              if (active) { add(nf, nt, mk('cm-lp-wikisrc')); add(nt, nt, Decoration.widget({ widget: w, side: 1 })); }
              else add(nf, nt, Decoration.replace({ widget: w }));
            } else if (active) {
              add(nf, nt, mk('cm-lp-wikisrc'));
            } else {
              add(nf, nt, Decoration.replace({ widget: new LinkWidget(label || target, target, true) }));
            }
            return false;
          }
        }
      },
    });
    // {{ассеты}} — плашкой везде (в тексте, не в коде).
    const text = doc.sliceString(from, to);
    const re = /\{\{\s*([^{}]+?)\s*\}\}/g;
    let m;
    while ((m = re.exec(text))) add(from + m.index, from + m.index + m[0].length, mk('cm-lp-asset'));
  }
  return Decoration.set(out, true);
}

export function livePreview(opts) {
  const plugin = ViewPlugin.fromClass(class {
    constructor(view) { this.decorations = build(view, opts); }
    update(u) {
      if (u.docChanged || u.viewportChanged || u.selectionSet || u.focusChanged || u.transactions.some((t) => t.effects.length) || syntaxTree(u.startState) !== syntaxTree(u.state)) {
        this.decorations = build(u.view, opts);
      }
    }
  }, { decorations: (v) => v.decorations });

  // Клик по отрендеренной ссылке открывает её (на строке с курсором — только с Ctrl/Cmd).
  const clicks = EditorView.domEventHandlers({
    mousedown(e, view) {
      const el = e.target.closest && e.target.closest('[data-href]');
      if (!el || e.button !== 0) return false;
      const isWidget = !!el.dataset.wiki;
      const pos = view.posAtDOM(el);
      const onActive = activeLines(view).has(view.state.doc.lineAt(pos).number);
      if (onActive && !(e.ctrlKey || e.metaKey) && !isWidget) return false;
      e.preventDefault();
      opts.onOpenLink && opts.onOpenLink(el.dataset.href, !!el.dataset.wiki);
      return true;
    },
  });
  return [plugin, clicks];
}
