import fs from 'fs'
import path from 'path'
import YAML from 'yaml'

// Parse a JSON or YAML file by extension. .yml/.yaml → YAML, anything else → JSON.
export function parseConfigFile(filePath: string): unknown {
  const raw = fs.readFileSync(filePath, { encoding: 'utf-8' })
  const ext = path.extname(filePath).toLowerCase()
  if (ext === '.yml' || ext === '.yaml') {
    return YAML.parse(raw)
  }

  return JSON.parse(raw)
}

export async function timeout(ms: number) {
  return new Promise(resolve => {
    setTimeout(resolve, ms)
  })
}

export function chunkArray<T>(array: T[], chunkSize: number): T[][] {
  const result: T[][] = []
  for (let i = 0; i < array.length; i += chunkSize) {
    result.push(array.slice(i, i + chunkSize))
  }

  return result
}
