#!/usr/bin/env python3
"""Реплей действий, снятых с реального движка, на эмуляторе + побайтовый дифф
каждой страницы со снимком (emudiff.py).

    python replay.py <steps.txt> [--base http://127.0.0.1:8090] [--snapshots data/emu/snapshots] [--filter L08]

Формат steps.txt (по строке на шаг): <имя-снимка> <get|post|q|goto|reset> [аргумент]
    L06-before get
    L06-wrong  post неверный код
    L11-open   q    pact=2&pid=11201
    -          goto 8
    -          reset
"""
import argparse
import os
import re
import subprocess
import sys
import urllib.parse
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("steps")
    ap.add_argument("--base", default="http://127.0.0.1:8090")
    ap.add_argument("--snapshots", default="data/emu/snapshots")
    ap.add_argument("--filter", default="")
    ap.add_argument("--lines", type=int, default=40)
    a = ap.parse_args()
    try:
        sys.stdout.reconfigure(encoding="utf-8")
    except Exception:
        pass

    gid = None
    ok = bad = 0
    out_dir = os.path.join(os.environ.get("TEMP", "/tmp"), "emu-replay")
    os.makedirs(out_dir, exist_ok=True)

    def play():
        nonlocal gid
        if gid is None:
            with urllib.request.urlopen(a.base + "/", ) as r:
                gid = r.url.rstrip("/").rsplit("/", 1)[-1]
        return f"{a.base}/gameengines/encounter/play/{gid}/"

    for raw in open(a.steps, encoding="utf-8"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split(None, 2)
        name, kind, arg = parts[0], parts[1], (parts[2] if len(parts) > 2 else "")
        body = None
        if kind == "reset":
            page = urllib.request.urlopen(play()).read().decode("utf-8")
            m = re.search(r'name="LevelNumber" value="(\d+)"', page)
            n = m.group(1) if m else "1"
            req = urllib.request.Request(a.base + "/api/state/" + n, data=b'{"action":"resetAll"}', headers={"Content-Type": "application/json"})
            urllib.request.urlopen(req).read()
            continue
        if kind == "goto":
            urllib.request.urlopen(a.base + "/play/" + arg).read()
            continue
        if kind == "get":
            req = urllib.request.Request(play())
        elif kind == "q":
            req = urllib.request.Request(play() + "?" + arg)
        elif kind == "post":
            data = urllib.parse.urlencode({"LevelAction.Answer": arg}).encode("utf-8")
            req = urllib.request.Request(play(), data=data, headers={"Content-Type": "application/x-www-form-urlencoded"})
        else:
            print("?? неизвестный шаг:", line)
            continue
        html = urllib.request.urlopen(req).read().decode("utf-8")
        out = os.path.join(out_dir, name + ".html")
        open(out, "w", encoding="utf-8", newline="").write(html)
        if a.filter and a.filter not in name:
            continue
        snap = os.path.join(a.snapshots, name + ".html")
        if not os.path.exists(snap):
            print(f"??  {name}: нет снимка {snap}")
            continue
        res = subprocess.run([sys.executable, os.path.join(HERE, "emudiff.py"), snap, out], capture_output=True, text=True, encoding="utf-8")
        if res.returncode == 0:
            print(f"OK  {name}")
            ok += 1
        else:
            print("\n".join("    " + l for l in res.stdout.splitlines()[: a.lines]))
            print(f"BAD {name}")
            bad += 1
    print(f"=== OK {ok} / BAD {bad}")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
