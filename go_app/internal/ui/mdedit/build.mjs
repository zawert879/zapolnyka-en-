// Сборка бандла md-редактора: один IIFE-файл с глобальным MdEdit, без внешних зависимостей.
import { build } from 'esbuild';

await build({
  entryPoints: ['src/index.js'],
  bundle: true,
  minify: true,
  format: 'iife',
  globalName: 'MdEdit',
  target: ['es2020'],
  legalComments: 'eof',
  outfile: '../static/vendor/mdedit/mdedit.js',
});
console.log('ok → ../static/vendor/mdedit/mdedit.js');
