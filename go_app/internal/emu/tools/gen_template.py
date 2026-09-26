#!/usr/bin/env python3
"""Генерирует templates/play.html из снимка реальной play-страницы en.cx.

Снимок должен быть с уровня БЕЗ секторов/подсказок/бонусов/таймера, но с телом
задания (например data/emu/snapshots/L06-before.html). Динамические места
заменяются плейсхолдерами html/template; всё остальное — включая хвостовые
табы, пустые строки и CRLF-переводы строк движка — сохраняется байт в байт.

    python gen_template.py data/emu/snapshots/L06-before.html [templates/play.html]
"""
import os
import re
import sys

src = sys.argv[1]
dst = sys.argv[2] if len(sys.argv) > 2 else os.path.join(os.path.dirname(__file__), "..", "templates", "play.html")

html = open(src, encoding="utf-8", newline="").read()
NL = "\r\n"  # движок отдаёт CRLF; тело задания вставляется как есть (обычно LF)


def need(pattern, repl, flags=0):
    global html
    new, n = re.subn(pattern, repl, html, count=0, flags=flags)
    if n == 0:
        sys.exit(f"не найдено: {pattern!r}")
    html = new


gid = re.search(r"/gameengines/encounter/play/(\d+)/", html).group(1)
level = re.search(r'name="LevelNumber" value="(\d+)"', html).group(1)
level_id = re.search(r'name="LevelId" value="(\d+)"', html).group(1)
title = re.search(r'id="lblGameTitle">([^<]*)</a>', html).group(1)

# --- голова: CSS/JS движка, версия, toastr
need(r"https://world\.en\.cx", "{{.EngineBase}}")
need(r"\?ver=[0-9.]+", "?ver={{.EngineVer}}")
need(r"//cdnjs\.cloudflare\.com/ajax/libs/toastr\.js/latest", "{{.ToastrBase}}")

# --- игра и уровень
need(re.escape(f"/gameengines/encounter/play/{gid}/"), "{{.PlayPath}}")
need(re.escape(f"gid={gid}"), "gid={{.GameID}}")
need(re.escape(f"/print/{gid}/"), "/print/{{.GameID}}/")
need(re.escape(f'id="lblGameTitle">{title}</a>'), 'id="lblGameTitle">{{.GameTitle}}</a>')
need(r"rnd=0,\d+", "rnd={{.Rnd}}")
need(re.escape(f"level={level}'"), "level={{.Number}}'")
need(re.escape(f"?level={level}\""), "?level={{.Number}}\"")
need(r"topic=\d+", "topic={{.Topic}}")
need(re.escape(f'name="LevelId" value="{level_id}"'), 'name="LevelId" value="{{.LevelID}}"')
need(re.escape(f'name="LevelNumber" value="{level}"'), 'name="LevelNumber" value="{{.Number}}"')
need(r'autocomplete="off" value="" />', 'autocomplete="off" {{.AnswerAttr}} />')

# --- история: <ul>, [уведомление], пустая строка, записи, пустая строка, </ul>
need(re.escape('<ul class="history">' + NL + NL + NL + "\t\t</ul>"),
     '<ul class="history">' + NL + "{{.HistoryHTML}}\t\t</ul>")

# --- заголовок уровня
need(re.escape(f"<h2>Уровень <span>{level}</span> из ") + r"\d+(: [^<]*)?</h2>",
     "<h2>Уровень <span>{{.Number}}</span> из {{.LevelsTotal}}{{if .LevelName}}: {{.LevelNameHTML}}{{end}}</h2>")

# --- блоки после h2: 3 пустые строки, таймер, сектора, пустая строка, задание
need(re.escape("</h2>" + NL + NL + NL + NL + NL + '\t<div class="spacer"></div>' + NL + "\t<h3>Задание</h3>" + NL + '\t<div class="task">' + NL + "\t<p>")
     + r".*?" + re.escape("</p>" + NL + "\t</div>" + NL + NL + '<div id="ordinary_helps">'),
     "</h2>" + NL + NL + NL + NL + "{{.TimerHTML}}{{.SectorsHTML}}" + NL + '{{.TaskBlockHTML}}<div id="ordinary_helps">', flags=re.S)
need(re.escape('<div id="ordinary_helps">\t' + NL + "</div>"), '<div id="ordinary_helps">\t' + NL + "{{.HelpsHTML}}</div>")
need(re.escape('<div id="penalty_helps">' + NL + "</div>"), '<div id="penalty_helps">' + NL + "{{.PenaltiesHTML}}</div>")
need(re.escape('<div id="bonuses">' + NL + NL + "</div>"), '<div id="bonuses">' + NL + "{{.BonusesHTML}}</div>")

# --- футер: ссылки EN/RU несут query текущего запроса (langHref в model.go)
need(r'<a href="\{\{\.PlayPath\}\}\?lang=en" ', '<a href="{{.LangHrefEN}}" ')
need(r'<a href="\{\{\.PlayPath\}\}\?lang=ru" ', '<a href="{{.LangHrefRU}}" ')

# --- панель эмулятора после </html> (браузер добавит её в body; из сравнения вырезается)
if not html.endswith("</html>"):
    sys.exit("снимок не заканчивается на </html>")
html += "{{.PanelHTML}}"

if gid in html or title in html:
    sys.exit("в шаблоне остались значения конкретной игры")

open(dst, "w", encoding="utf-8", newline="").write(html)
print(f"ok: {dst} ({len(html)} байт, CR={html.count(chr(13))}) из {src}")
