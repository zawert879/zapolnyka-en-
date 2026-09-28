// MdEdit — markdown-редактор заметок автора для веб-интерфейса (CodeMirror 6 + Live Preview).
//
//   const ed = MdEdit.create(el, {
//     doc, resolve(url, wiki) → src|'' , onChange(text), onOpenLink(href, wiki),
//     onDrop(dataTransfer) → текст|Promise<текст>|null,   // перетаскивание в редактор
//     onPasteFiles(files) → Promise<текст>|null,           // вставка картинок из буфера
//   });
//   ed.open(key, text) · ed.forget(key) · ed.setDoc(text) · ed.getDoc() · ed.insert(text) · ed.focus() · ed.refresh() · ed.destroy()
import { EditorState, Compartment, StateEffect } from '@codemirror/state';
import { EditorView, keymap, drawSelection, highlightActiveLine, placeholder, dropCursor } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { syntaxHighlighting, HighlightStyle, indentOnInput, bracketMatching } from '@codemirror/language';
import { markdown, markdownLanguage, markdownKeymap } from '@codemirror/lang-markdown';
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
import { closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete';
import { tags as t } from '@lezer/highlight';
import { livePreview, wikiLinks } from './livepreview.js';
import { tables } from './tables.js';

const highlight = HighlightStyle.define([
  { tag: t.heading, color: '#e7e9ee', fontWeight: '700' },
  { tag: t.strong, fontWeight: '700' },
  { tag: t.emphasis, fontStyle: 'italic' },
  { tag: t.strikethrough, textDecoration: 'line-through' },
  { tag: [t.link, t.url], color: '#7fb2ff' },
  { tag: t.monospace, fontFamily: 'var(--mono)', color: '#e5c07b' },
  { tag: t.quote, color: '#b7bcc7' },
  { tag: [t.processingInstruction, t.meta, t.contentSeparator], color: '#6b7280' },
  { tag: t.list, color: '#9aa0ab' },
]);

const theme = EditorView.theme({
  '&': { height: '100%', color: '#e7e9ee', backgroundColor: '#101114', fontSize: '15px' },
  '.cm-scroller': { fontFamily: 'Manrope, "Segoe UI", Roboto, Arial, sans-serif', lineHeight: '1.6' },
  '.cm-content': { maxWidth: '860px', margin: '0 auto', padding: '24px 32px 40vh', caretColor: '#35a85a' },
  '.cm-cursor': { borderLeftColor: '#35a85a', borderLeftWidth: '2px' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': { backgroundColor: '#2d4d7a !important' },
  '.cm-activeLine': { backgroundColor: 'transparent' },
  '.cm-placeholder': { color: '#5d636e' },
  '.cm-lp-h1': { fontSize: '1.9em', fontWeight: '800', lineHeight: '1.3', paddingTop: '.4em' },
  '.cm-lp-h2': { fontSize: '1.5em', fontWeight: '800', lineHeight: '1.3', paddingTop: '.35em' },
  '.cm-lp-h3': { fontSize: '1.25em', fontWeight: '700', paddingTop: '.3em' },
  '.cm-lp-h4, .cm-lp-h5, .cm-lp-h6': { fontSize: '1.08em', fontWeight: '700' },
  '.cm-lp-strong': { fontWeight: '700' },
  '.cm-lp-em': { fontStyle: 'italic' },
  '.cm-lp-strike': { textDecoration: 'line-through', color: '#9aa0ab' },
  '.cm-lp-code': { fontFamily: 'var(--mono)', fontSize: '.88em', background: '#1d1f24', borderRadius: '4px', padding: '1px 4px', color: '#e5c07b' },
  '.cm-lp-link': { color: '#7fb2ff', textDecoration: 'underline', textDecorationColor: '#7fb2ff66', cursor: 'pointer' },
  '.cm-lp-wiki': { textDecoration: 'none', borderBottom: '1px dashed #7fb2ff88' },
  '.cm-lp-wikisrc': { color: '#7fb2ff' },
  '.cm-lp-asset': { fontFamily: 'var(--mono)', fontSize: '.85em', background: '#1d3a26', color: '#7fdc9a', borderRadius: '4px', padding: '1px 4px' },
  '.cm-lp-quote': { borderLeft: '3px solid #35a85a', paddingLeft: '14px !important', color: '#b7bcc7' },
  '.cm-lp-codeblock': { fontFamily: 'var(--mono)', fontSize: '13px', background: '#17181c', paddingLeft: '14px !important' },
  '.cm-lp-cb-first': { borderRadius: '6px 6px 0 0' },
  '.cm-lp-cb-last': { borderRadius: '0 0 6px 6px' },
  '.cm-lp-fence': { color: '#4b515c', fontSize: '11px' },
  '.cm-lp-table': { fontFamily: 'var(--mono)', fontSize: '13px' },
  '.cm-lp-bullet': { color: '#35a85a', fontWeight: '800', padding: '0 4px 0 2px' },
  '.cm-lp-task': { width: '15px', height: '15px', margin: '0 6px 0 0', verticalAlign: '-2px', accentColor: '#35a85a', cursor: 'pointer' },
  '.cm-lp-done': { color: '#6b7280', textDecoration: 'line-through' },
  '.cm-lp-hr': { display: 'inline-block', width: '100%', borderTop: '1px solid #2a2d34', verticalAlign: 'middle' },
  '.cm-lp-imgwrap': { display: 'inline-block', maxWidth: '100%', verticalAlign: 'top' },
  '.cm-lp-img': { maxWidth: '100%', maxHeight: '520px', borderRadius: '6px', display: 'block', margin: '6px 0' },
  '.cm-lp-broken': { display: 'inline-block', color: '#e5484d', fontSize: '12px', fontFamily: 'var(--mono)', background: '#2a1416', borderRadius: '4px', padding: '1px 6px' },
  '.cm-dropCursor': { borderLeftColor: '#35a85a' },
  '.cm-lp-tablewrap': { overflowX: 'auto', padding: '4px 0', cursor: 'text' },
  '.cm-lp-tbl': { borderCollapse: 'collapse', fontSize: '14px', lineHeight: '1.45' },
  '.cm-lp-tbl th, .cm-lp-tbl td': { border: '1px solid #2a2d34', padding: '5px 12px', verticalAlign: 'top' },
  '.cm-lp-tbl th': { background: '#17181c', fontWeight: '700', textAlign: 'left' },
  '.cm-lp-tbl tbody tr:nth-child(even) td': { background: '#131418' },
  '.cm-lp-cellimg': { maxWidth: '240px', maxHeight: '160px', borderRadius: '4px', verticalAlign: 'middle' },
}, { dark: true });

const refreshEffect = StateEffect.define();

export function create(el, opts = {}) {
  const editable = new Compartment();
  let readOnly = false;
  const handlers = EditorView.domEventHandlers({
    drop(e, view) {
      if (!opts.onDrop) return false;
      const pos = view.posAtCoords({ x: e.clientX, y: e.clientY });
      const res = opts.onDrop(e.dataTransfer);
      if (res == null) return false;
      e.preventDefault();
      const at = pos == null ? view.state.selection.main.head : pos;
      Promise.resolve(res).then((text) => {
        if (!text) return;
        view.dispatch({ changes: { from: at, insert: text }, selection: { anchor: at + text.length } });
        view.focus();
      });
      return true;
    },
    paste(e, view) {
      if (!opts.onPasteFiles) return false;
      const files = [...(e.clipboardData ? e.clipboardData.files : [])];
      if (!files.length) return false;
      const res = opts.onPasteFiles(files);
      if (res == null) return false;
      e.preventDefault();
      Promise.resolve(res).then((text) => { if (text) insert(view, text); });
      return true;
    },
  });
  const lp = { resolve: (u, w) => (opts.resolve ? opts.resolve(u, w) : u), onOpenLink: (h, w) => opts.onOpenLink && opts.onOpenLink(h, w) };
  const state = (doc) => EditorState.create({
    doc,
    extensions: [
      history(),
      drawSelection(),
      dropCursor(),
      indentOnInput(),
      bracketMatching(),
      closeBrackets(),
      highlightActiveLine(),
      highlightSelectionMatches(),
      EditorView.lineWrapping,
      markdown({ base: markdownLanguage, extensions: [wikiLinks] }),
      syntaxHighlighting(highlight),
      livePreview(lp),
      tables(lp),
      keymap.of([...closeBracketsKeymap, ...markdownKeymap, ...defaultKeymap, ...searchKeymap, ...historyKeymap, indentWithTab]),
      placeholder(opts.placeholder || ''),
      editable.of(EditorView.editable.of(!readOnly)),
      handlers,
      theme,
      EditorView.updateListener.of((u) => { if (u.docChanged && !syncing && opts.onChange) opts.onChange(u.state.doc.toString()); }),
    ],
  });
  let syncing = false, curKey = null;
  const states = new Map();
  const view = new EditorView({ state: state(opts.doc || ''), parent: el });
  function insert(v, text) {
    const r = v.state.selection.main;
    v.dispatch({ changes: { from: r.from, to: r.to, insert: text }, selection: { anchor: r.from + text.length } });
    v.focus();
  }
  return {
    view,
    // setDoc заменяет документ целиком (с новой историей правок), без onChange.
    setDoc(text) {
      if (view.state.doc.toString() === text) return;
      syncing = true;
      try { view.setState(state(text)); } finally { syncing = false; }
    },
    // open показывает файл key: состояние (история, курсор) по ключу запоминается,
    // пока текст на диске не поменялся в обход редактора.
    open(key, text) {
      if (curKey != null) states.set(curKey, view.state);
      let st = states.get(key);
      if (!st || st.doc.toString() !== text) st = state(text);
      curKey = key;
      syncing = true;
      try { view.setState(st); } finally { syncing = false; }
    },
    forget(key) { states.delete(key); if (curKey === key) curKey = null; },
    getDoc: () => view.state.doc.toString(),
    insert: (text) => insert(view, text),
    setReadOnly(ro) { readOnly = !!ro; view.dispatch({ effects: editable.reconfigure(EditorView.editable.of(!readOnly)) }); },
    focus: () => view.focus(),
    refresh: () => view.dispatch({ effects: refreshEffect.of(null) }), // перерисовать (например, изменился список файлов)
    destroy: () => view.destroy(),
  };
}
