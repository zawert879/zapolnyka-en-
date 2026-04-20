import { type z } from 'zod'
import { Config } from './config'

/* eslint-disable @typescript-eslint/no-namespace */
export namespace Data {
  export namespace Config {
    export type Self = z.infer<typeof Config.Self>;
    export type Game = z.infer<typeof Config.Game>;
    export type Level = z.infer<typeof Config.Level>;
  }
}
export { Config }
