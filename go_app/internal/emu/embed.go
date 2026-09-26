package emu

import "embed"

// embedded — шаблоны страницы и статика (файлы движка en.cx, панель).
//
//go:embed all:static all:templates
var embedded embed.FS
