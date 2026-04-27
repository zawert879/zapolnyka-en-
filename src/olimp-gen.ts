import * as fs from 'fs';
import * as path from 'path';

interface Word {
  id: number;
  word: string;
  from: number[];
}

interface Bracket {
  level: number;
  N: number;
  prefix: string;
  champion: string;
  words: Word[];
}

function roundStartTableId(N: number, r: number): number {
  let sum = 0;
  for (let j = 1; j <= r - 2; j++) sum += N / Math.pow(2, j);
  return N + 1 + sum;
}

function generateTaskHtml(b: Bracket): string {
  const { N, prefix, words } = b;
  const numRounds = Math.log2(N);

  const rows: string[] = [];

  for (let i = 0; i < N; i++) {
    const cells: string[] = [];
    cells.push(`  <td id="${prefix}_${i + 1}"><p class="up">${words[i].word}</p></td>`);

    for (let r = 2; r <= numRounds + 1; r++) {
      const stepSize = Math.pow(2, r - 1);
      if (i % stepSize === 0) {
        const pairIndex = i / stepSize;
        const tableId = roundStartTableId(N, r) + pairIndex;
        cells.push(`  <td id="${prefix}_${tableId}" rowspan="${stepSize}">&nbsp;</td>`);
      }
    }

    rows.push(`<tr>\n${cells.join('\n')}\n</tr>`);
  }

  return `<style>
.olymp { border-collapse: collapse; margin: 10px auto; }
.olymp td { border: 1px solid white; width: 140px; text-align: center; vertical-align: middle; padding: 4px; font-family: sans-serif; font-size: 12px; }
.up { color: #0F0; font-weight: bold; margin: 0; }
.timer { display: block!important; }
.cols-wrapper { display: none; }
.content h3 { display: none; }
.color_dis { display: none; }
</style>

<table class="olymp">
${rows.join('\n')}
</table>
`;
}

function generateCodesYml(b: Bracket): string {
  const { N, prefix, words } = b;
  const entries: string[] = [];

  for (let j = N; j <= 2 * N - 2; j++) {
    const tableId = j + 1;
    const word = words[j].word;
    // Single quotes in YAML are escaped by doubling: '' inside '...'
    const help = `'<script>document.getElementById("${prefix}_${tableId}").innerHTML=''<p class="up">${word}</p>'';</script>'`;

    entries.push(
      `- type: секторбонус\n` +
      `  sectorName: 'Матч ${tableId}'\n` +
      `  bonusName: 'Матч ${tableId}'\n` +
      `  answers:\n` +
      `    - ${word}\n` +
      `  time: 30\n` +
      `  help: ${help}`,
    );
  }

  return entries.join('\n\n') + '\n';
}

function generateConfYml(b: Bracket): string {
  return (
    `level: ${b.level}\n` +
    `codes: codes.yml\n` +
    `body: task.html\n` +
    `clean: true\n` +
    `sectorsToClose: ${b.N - 1}\n`
  );
}

function main(): void {
  const inputFile = process.argv[2];

  if (!inputFile) {
    console.error('Usage: ts-node src/olimp-gen.ts <path/to/bracket.json>');
    process.exit(1);
  }

  if (!fs.existsSync(inputFile)) {
    console.error(`File not found: ${inputFile}`);
    process.exit(1);
  }

  const bracket: Bracket = JSON.parse(fs.readFileSync(inputFile, 'utf-8')) as Bracket;
  const { level, N, words } = bracket;

  if (!Number.isInteger(Math.log2(N))) {
    console.error(`N must be a power of 2, got ${N}`);
    process.exit(1);
  }

  if (words.length !== 2 * N - 1) {
    console.error(`Expected ${2 * N - 1} words, got ${words.length}`);
    process.exit(1);
  }

  const dir = path.dirname(path.resolve(inputFile));

  fs.writeFileSync(path.join(dir, 'task.html'), generateTaskHtml(bracket));
  fs.writeFileSync(path.join(dir, 'codes.yml'), generateCodesYml(bracket));
  fs.writeFileSync(path.join(dir, 'conf.yml'), generateConfYml(bracket));

  console.log(`Level ${level} (N=${N}, champion: ${bracket.champion})`);
  console.log(`  task.html  — ${N} rows, ${Math.log2(N) + 1} columns`);
  console.log(`  codes.yml  — ${N - 1} entries`);
  console.log(`  conf.yml   — sectorsToClose: ${N - 1}`);
  console.log(`  → ${dir}`);
}

main();
