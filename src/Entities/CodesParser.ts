/* eslint-disable @typescript-eslint/no-unsafe-call */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import Table from 'cli-table'
import colors from 'colors/safe'
import z from 'zod'
import { parseConfigFile } from '../utils'

const CODE_TYPES = ['сектор', 'бонус', 'штраф', 'секторбонус', 'секторштраф'] as const
export type CodeType = typeof CODE_TYPES[number]

export const CodeSchema = z.object({
  type: z.enum(CODE_TYPES),
  sectorName: z.string().optional(), // имя сектора в админке; для типов с сектором
  bonusName: z.string().optional(),  // имя бонуса в админке; для типов с бонусом/штрафом
  answers: z.array(z.string().min(1)).min(1),
  time: z.number().int().nonnegative().optional(),
  help: z.string().optional(),
}).superRefine((c, ctx) => {
  const isSector = c.type === 'сектор' || c.type === 'секторбонус' || c.type === 'секторштраф'
  const isBonus = c.type !== 'сектор'
  if (isBonus && c.time === undefined) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, message: `time обязательно для типа ${c.type}` })
  }

  // sectorName/bonusName опциональны — по умолчанию en.cx сам сгенерирует имя
  void isSector
})

export const CodesSchema = z.array(CodeSchema)
export type Code = z.infer<typeof CodeSchema>

export class CodesParser {
  go(filePath: string): Code[] {
    const data = parseConfigFile(filePath)
    const parsed = CodesSchema.parse(data)

    const table = new Table({
      head: ['Тип', 'Сектор', 'Бонус', 'Время', 'Ответы', 'Help'],
    })
    for (const c of parsed) {
      const typeColored = c.type === 'сектор' ? colors.green(c.type)
        : c.type === 'штраф' ? colors.red(c.type)
        : c.type === 'бонус' ? colors.yellow(c.type)
        : c.type === 'секторбонус' ? colors.cyan(c.type)
        : colors.magenta(c.type)
      table.push([
        typeColored,
        c.sectorName ?? '',
        c.bonusName ?? '',
        c.time !== undefined ? `${c.time}s` : '-',
        c.answers.join('\n'),
        c.help !== undefined ? `${c.help.length}ch` : '',
      ])
    }

    console.log(table.toString())
    return parsed
  }
}
