#!/usr/bin/env node
/**
 * Cross-checks the frontend's text helpers against the shared contract in
 * scripts/text-cases.json.
 *
 * The Go side asserts the very same file in
 * backend/internal/pkg/textutil/textutil_test.go, so a green run here plus a
 * green `go test ./internal/pkg/textutil` proves the two implementations agree.
 *
 * Run with: node scripts/check-text-contract.mjs
 * (Node 22.18+ strips the TypeScript types natively.)
 */
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { autoExcerpt, countWords, readingMinutes, slugify } from '../src/utils/text.ts'

const here = dirname(fileURLToPath(import.meta.url))
const spec = JSON.parse(readFileSync(join(here, 'text-cases.json'), 'utf8'))

const build = (parts) => (parts ?? []).map((part) => part.unit.repeat(part.count)).join('')

let failures = 0

for (const item of spec.cases) {
  const input = item.inputBuild ? build(item.inputBuild) : item.input

  const expectedSlug = item.slug ?? (item.slugBuild ? build(item.slugBuild) : undefined)

  const checks = [
    ['slug', expectedSlug, () => slugify(input)],
    ['words', item.words, () => countWords(input)],
    ['minutes', item.minutes, () => readingMinutes(input)],
    ['excerpt', item.excerpt, () => autoExcerpt(input, spec.excerptLimit)],
  ]

  for (const [label, expected, compute] of checks) {
    if (expected === undefined) continue
    const actual = compute()
    if (actual !== expected) {
      failures += 1
      console.log(
        `FAIL ${item.name}.${label}\n` +
          `  expected: ${JSON.stringify(expected)}\n` +
          `  actual:   ${JSON.stringify(actual)}`,
      )
    }
  }
}

if (failures > 0) {
  console.log(`\n${failures} FAILURE(S)`)
  process.exit(1)
}

console.log(`OK - frontend text helpers match all ${spec.cases.length} contract cases`)
