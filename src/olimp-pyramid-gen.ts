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
  type?: string;
  champion: string;
  words: Word[];
}

// Pyramid layout: 4 quadrants of 8 starters each, champion has 4 parents.
// Q1 (top-left):     starters id 0-7,   derived id 32-38, winner id 38
// Q2 (top-right):    starters id 8-15,  derived id 39-45, winner id 45
// Q3 (bottom-right): starters id 16-23, derived id 46-52, winner id 52
// Q4 (bottom-left):  starters id 24-31, derived id 53-59, winner id 59
// Champion: id 60, from [38, 45, 52, 59]

function starterCell(prefix: string, id: number, word: string): string {
  return `<td id="${prefix}_${id + 1}"><p class="up">${word}</p></td>`;
}

function derivedCell(prefix: string, id: number, rowspan: number): string {
  const rs = rowspan > 1 ? ` rowspan="${rowspan}"` : '';
  return `<td id="${prefix}_${id + 1}"${rs}>&nbsp;</td>`;
}

function emptyTd(): string {
  return `<td>&nbsp;</td>`;
}

function generateTaskHtml(b: Bracket): string {
  const { prefix, words } = b;
  const rows: string[] = [];

  // Top section: rows 0-7
  // Cols: [Q1-starter] [Q1-R1?] [Q1-R2?] [Q1-win?] [empty] [Q2-win?] [Q2-R2?] [Q2-R1?] [Q2-starter]
  for (let i = 0; i < 8; i++) {
    const cells: string[] = [];

    cells.push(starterCell(prefix, i, words[i].word));

    if (i % 2 === 0) cells.push(derivedCell(prefix, 32 + i / 2, 2));
    if (i % 4 === 0) cells.push(derivedCell(prefix, 36 + i / 4, 4));
    if (i === 0)     cells.push(derivedCell(prefix, 38, 8));

    cells.push(emptyTd());

    if (i === 0)     cells.push(derivedCell(prefix, 45, 8));
    if (i % 4 === 0) cells.push(derivedCell(prefix, 43 + i / 4, 4));
    if (i % 2 === 0) cells.push(derivedCell(prefix, 39 + i / 2, 2));

    cells.push(starterCell(prefix, 8 + i, words[8 + i].word));

    rows.push(`<tr>${cells.join('')}</tr>`);
  }

  // Center row: champion in col 4
  {
    const cells: string[] = [];
    for (let col = 0; col < 9; col++) {
      cells.push(col === 4 ? derivedCell(prefix, 60, 1) : emptyTd());
    }
    rows.push(`<tr>${cells.join('')}</tr>`);
  }

  // Bottom section: rows 9-16
  // Cols: [Q4-starter] [Q4-R1?] [Q4-R2?] [Q4-win?] [empty] [Q3-win?] [Q3-R2?] [Q3-R1?] [Q3-starter]
  for (let i = 0; i < 8; i++) {
    const cells: string[] = [];

    cells.push(starterCell(prefix, 24 + i, words[24 + i].word));

    if (i % 2 === 0) cells.push(derivedCell(prefix, 53 + i / 2, 2));
    if (i % 4 === 0) cells.push(derivedCell(prefix, 57 + i / 4, 4));
    if (i === 0)     cells.push(derivedCell(prefix, 59, 8));

    cells.push(emptyTd());

    if (i === 0)     cells.push(derivedCell(prefix, 52, 8));
    if (i % 4 === 0) cells.push(derivedCell(prefix, 50 + i / 4, 4));
    if (i % 2 === 0) cells.push(derivedCell(prefix, 46 + i / 2, 2));

    cells.push(starterCell(prefix, 16 + i, words[16 + i].word));

    rows.push(`<tr>${cells.join('')}</tr>`);
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
  const { prefix, words } = b;
  const entries: string[] = [];

  // Derived words: id 32-60 (29 entries)
  for (let j = 32; j <= 60; j++) {
    const tableId = j + 1;
    const word = words[j].word;
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
  // 29 derived words (4×7 quadrant + 1 champion)
  const numDerived = 29;
  return (
    `level: ${b.level}\n` +
    `codes: codes.yml\n` +
    `body: task.html\n` +
    `clean: true\n` +
    `sectorsToClose: ${numDerived}\n`
  );
}

function main(): void {
  const inputFile = process.argv[2];

  if (!inputFile) {
    console.error('Usage: ts-node src/olimp-pyramid-gen.ts <path/to/bracket.json>');
    process.exit(1);
  }

  if (!fs.existsSync(inputFile)) {
    console.error(`File not found: ${inputFile}`);
    process.exit(1);
  }

  const b: Bracket = JSON.parse(fs.readFileSync(inputFile, 'utf-8')) as Bracket;

  if (b.N !== 32) {
    console.error(`Pyramid generator only supports N=32, got N=${b.N}`);
    process.exit(1);
  }

  if (b.words.length !== 61) {
    console.error(`Pyramid requires exactly 61 words (32+28+1), got ${b.words.length}`);
    process.exit(1);
  }

  const dir = path.dirname(path.resolve(inputFile));

  fs.writeFileSync(path.join(dir, 'task.html'), generateTaskHtml(b));
  fs.writeFileSync(path.join(dir, 'codes.yml'), generateCodesYml(b));
  fs.writeFileSync(path.join(dir, 'conf.yml'), generateConfYml(b));

  console.log(`Level ${b.level} pyramid (champion: ${b.champion})`);
  console.log(`  task.html  — 17 rows (8+1+8), 9 columns`);
  console.log(`  codes.yml  — 29 entries (id 32–60)`);
  console.log(`  conf.yml   — sectorsToClose: 29`);
  console.log(`  → ${dir}`);
}

main();
