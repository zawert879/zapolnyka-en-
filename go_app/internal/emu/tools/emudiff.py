#!/usr/bin/env python3
"""Сравнивает страницу эмулятора с реальной play-страницей en.cx побайтово,
маскируя только заведомо динамические места (случайные id, время, ids сущностей).

    python emudiff.py <real.html> <emu.html | http://127.0.0.1:8090/...>

Код возврата 0 — страницы совпадают. Что маскируется, см. NORMALIZE.
"""
import re
import sys
import urllib.request

NORMALIZE = [
    (r"rnd=0,\d+", "rnd=RND"),                                   # ссылка «Обновить»
    (r"time\d{5,}", "timeID"),                                   # id таймеров
    (r'"StartCounter":\d+', '"StartCounter":N'),                  # секунды таймера
    (r'(id="timeID">)[^<]*(</span>)', r"\1T\2"),                 # текст обратного отсчёта
    (r'name="LevelId" value="\d+"', 'name="LevelId" value="LID"'),
    (r'<p id="\d+">', '<p id="SID">'),                           # id секторов
    (r"pid=\d+", "pid=PID"),                                     # id штрафных подсказок
    (r"uid=\d+", "uid=UID"),
    (r"topic=\d+", "topic=TOPIC"),
    (r"\d\d\.\d\d\.\d{4} \d\d:\d\d:\d\d", "DD.MM.YYYY HH:MM:SS"),  # дата в секторе
    (r"\d\d\.\d\d \d\d:\d\d:\d\d", "DD.MM HH:MM:SS"),              # дата в бонусе
    (r"(?m)^\d\d:\d\d:\d\d(?=\r?$)", "HH:MM:SS"),                   # время в истории
    (r"(?s)\n<!--emu-->.*$", ""),                                  # dev-панель эмулятора
    # ассеты: на en.cx — uuid-имена на d1, в эмуляторе — локальные /assets/<имя>
    (r"https://(?:d1|cdn)\.endata\.cx/data/games/\d+/[0-9a-f-]{36}\.(\w+)", r"ASSET.\1"),
    (r"/assets/[A-Za-z0-9_~.-]+\.(\w+)", r"ASSET.\1"),
]


def load(src):
    if src.startswith("http://") or src.startswith("https://"):
        with urllib.request.urlopen(src) as r:
            return r.read().decode("utf-8")
    return open(src, encoding="utf-8", newline="").read()


def normalize(s):
    for pat, repl in NORMALIZE:
        s = re.sub(pat, repl, s)
    return s


def main():
    try:
        sys.stdout.reconfigure(encoding="utf-8")
    except Exception:
        pass
    real, emu = load(sys.argv[1]), load(sys.argv[2])
    a, b = normalize(real), normalize(emu)
    if a == b:
        print(f"OK: совпадает побайтово (после маскировки), {len(real)} байт")
        return 0
    al, bl = a.split("\n"), b.split("\n")
    print(f"DIFF: real {len(al)} строк / emu {len(bl)} строк")
    import difflib
    shown = 0
    for line in difflib.unified_diff(al, bl, "real", "emu", lineterm="", n=1):
        vis = line.replace("\t", "→").replace("\r", "␍")
        print(vis[:220])
        shown += 1
        if shown > 120:
            print("…")
            break
    return 1


if __name__ == "__main__":
    sys.exit(main())
