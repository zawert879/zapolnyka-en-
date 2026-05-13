/* eslint-disable @typescript-eslint/naming-convention */
/* eslint-disable @typescript-eslint/no-namespace */
import z from 'zod'

export namespace Config {
  export const Hint = z.object({
    time: z.number(), // seconds after level start
    text: z.string(),
  })

  export const PenaltyHint = z.object({
    time: z.number(), // seconds after level start (when hint becomes available)
    text: z.string(),
    penalty: z.number().optional(), // seconds; time penalty for opening (default 2:00:00 from server)
    comment: z.string().optional(), // warning text shown before opening
  })

  // Per-level config — no auth, no gameId (they come from game.json)
  export const Level = z.object({
    level: z.number(),
    devLevel: z.number().optional(), // номер уровня в dev-режиме (используется если game.isDev = true)
    codes: z.string().optional(), // path to codes.json
    body: z.string().optional(),
    clean: z.boolean().optional(),
    autopass: z.number().optional(),
    autopassPenalty: z.number().optional(),
    hints: z.array(Hint).optional(),
    penaltyHints: z.array(PenaltyHint).optional(),
    sectorsToClose: z.number().int().positive().optional(), // required sectors count; if omitted — «все секторы»
    name: z.string().optional(), // «Название уровня» в админке
    comment: z.string().optional(), // «Комментарий к уровню» в админке
  })

  // Global game config — domain/gameId + list of level conf paths.
  // login/password берутся из .env (EN_LOGIN / EN_PASSWORD); в game.* их класть не надо.
  export const Game = z.object({
    domain: z.string(),
    gameId: z.number(),
    levels: z.array(z.string()),
    defaultFormat: z.enum(['json', 'yml']).optional(), // default config format for `new` command
    isDev: z.boolean().optional(), // dev-режим: вместо level используется devLevel из conf
  })

  // Fully merged config passed to Zapolnyaka — auth + one level
  export const Self = z.object({
    login: z.string(),
    password: z.string(),
    domain: z.string(),
    gameId: z.number(),
    level: z.number(),
    codes: z.string().optional(),
    body: z.string().optional(),
    clean: z.boolean().optional(),
    autopass: z.number().optional(),
    autopassPenalty: z.number().optional(),
    hints: z.array(Hint).optional(),
    penaltyHints: z.array(PenaltyHint).optional(),
    sectorsToClose: z.number().int().positive().optional(),
    name: z.string().optional(),
    comment: z.string().optional(),
  })
}
