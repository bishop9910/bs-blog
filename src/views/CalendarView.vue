<script setup lang="ts">
import { computed, ref } from 'vue'
import { PhCaretLeft, PhCaretRight, PhCalendarBlank } from '@phosphor-icons/vue'
import {
  getDayDetail,
  getLunarDate,
  getLunarFestivals,
  getSolarTerms,
  isHoliday,
  isInLieu,
  isWorkday,
} from 'chinese-days'

/**
 * Calendar page, ported from the author's old site (G:\bishop9910-website)
 * and restyled onto the current token set.
 *
 * Everything is offline now: 法定节假日、调休安排、农历、节气 all come out of the
 * data bundled with chinese-days, so there is no fetch, no cache and no
 * network-degradation branch any more. If a year is not covered yet (no day in
 * it carries a Chinese holiday name) the page says so instead of pretending.
 */

interface DayExtra {
  term: string
  festival: string
}

interface CalendarCell {
  key: string
  day: number
  inMonth: boolean
  isToday: boolean
  isWeekend: boolean
  isRest: boolean
  isAdjust: boolean
  name: string
}

const WEEKDAYS = ['一', '二', '三', '四', '五', '六', '日']
const REST_DAYS = [0, 6]

const today = new Date()
const todayKey = keyOf(today)

const viewYear = ref(today.getFullYear())
const viewMonth = ref(today.getMonth())

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

function keyOf(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function shift(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days)
}

function isWeekend(date: Date): boolean {
  return REST_DAYS.includes(date.getDay())
}

/** chinese-days 的 name 形如 "National Day,国庆节,3"；普通工作日/周末只有英文名。 */
function holidayNameOf(key: string): string {
  const parts = getDayDetail(key).name.split(',')
  return parts.length > 1 ? (parts[1] ?? '').trim() : ''
}

/** 农历日；初一显示月份名（九月 而不是 初一）。 */
function lunarLabelOf(key: string): string {
  const lunar = getLunarDate(key)
  return lunar.lunarDay === 1 ? lunar.lunarMonCN : lunar.lunarDayCN
}

/** 节气与农历节日：只对当前月视图覆盖到的那 5~6 周做一次查询。 */
const extras = computed<Record<string, DayExtra>>(() => {
  const first = new Date(viewYear.value, viewMonth.value, 1)
  const offset = (first.getDay() + 6) % 7
  const daysInMonth = new Date(viewYear.value, viewMonth.value + 1, 0).getDate()
  const rows = Math.ceil((offset + daysInMonth) / 7)
  const start = shift(first, -offset)
  const end = shift(start, rows * 7 - 1)

  const map: Record<string, DayExtra> = {}
  for (let i = 0; i < rows * 7; i += 1) {
    map[keyOf(shift(start, i))] = { term: '', festival: '' }
  }
  for (const item of getSolarTerms(keyOf(start), keyOf(end))) {
    map[item.date] = { ...(map[item.date] ?? { term: '', festival: '' }), term: item.name }
  }
  for (const item of getLunarFestivals(keyOf(start), keyOf(end))) {
    const current = map[item.date] ?? { term: '', festival: '' }
    map[item.date] = { ...current, festival: item.name.join('、') }
  }
  return map
})

const cells = computed<CalendarCell[]>(() => {
  const first = new Date(viewYear.value, viewMonth.value, 1)
  const offset = (first.getDay() + 6) % 7
  const daysInMonth = new Date(viewYear.value, viewMonth.value + 1, 0).getDate()
  const rows = Math.ceil((offset + daysInMonth) / 7)
  const list: CalendarCell[] = []

  for (let i = 0; i < rows * 7; i += 1) {
    const date = shift(first, i - offset)
    const key = keyOf(date)
    const weekend = isWeekend(date)
    const rest = isHoliday(key)
    // 周末却要上班 —— 调休补班日。
    const adjust = weekend && isWorkday(key)
    const extra = extras.value[key] ?? { term: '', festival: '' }
    const holiday = holidayNameOf(key)
    const prefix = adjust ? '补班' : rest && !weekend && isInLieu(key) ? '调休' : ''
    const labelled = holiday ? (prefix ? `${prefix}·${holiday}` : holiday) : ''

    list.push({
      key,
      day: date.getDate(),
      inMonth: date.getMonth() === viewMonth.value,
      isToday: key === todayKey,
      isWeekend: weekend,
      isRest: rest,
      isAdjust: adjust,
      name: extra.term || extra.festival || labelled || lunarLabelOf(key),
    })
  }
  return list
})

const monthStats = computed(() => {
  const inMonth = cells.value.filter((cell) => cell.inMonth)
  const workdays = inMonth.filter((cell) => !cell.isRest).length
  return { workdays, rests: inMonth.length - workdays, total: inMonth.length }
})

const nextRest = computed(() => {
  for (let i = 0; i < 400; i += 1) {
    const date = shift(today, i)
    const key = keyOf(date)
    if (isHoliday(key)) {
      const holiday = holidayNameOf(key)
      return { date, offset: i, name: holiday || (isWeekend(date) ? '周末' : '休息日') }
    }
  }
  return null
})

const todayInfo = computed(() => {
  const lunar = getLunarDate(todayKey)
  const term =
    getSolarTerms(keyOf(shift(today, -1)), keyOf(shift(today, 1))).find(
      (item) => item.date === todayKey,
    )?.name ?? ''
  return {
    date: today,
    week: isoWeek(today),
    dayNumber: dayOfYear(today),
    rest: isHoliday(todayKey),
    adjust: isWorkday(todayKey) && isWeekend(today),
    name: holidayNameOf(todayKey) || term,
    lunar: `${lunar.lunarMonCN}${lunar.lunarDayCN}`,
  }
})

const monthLabel = computed(() => `${viewYear.value} 年 ${viewMonth.value + 1} 月`)

const countdown = computed(() => {
  const next = nextRest.value
  if (!next) return '未来一年没有休息日'
  if (next.offset === 0) return '今天就是休息日'
  return `还有 ${next.offset} 天`
})

/** 这一年到底有没有被 chinese-days 收录法定安排。 */
const covered = computed(() => {
  const year = viewYear.value
  const cursor = new Date(year, 0, 1)
  while (cursor.getFullYear() === year) {
    if (holidayNameOf(keyOf(cursor))) return true
    cursor.setDate(cursor.getDate() + 1)
  }
  return false
})

const note = computed(() =>
  covered.value
    ? '假期与调休来自 chinese-days 内置数据，离线可用，不会因为接口挂掉而空白。'
    : `chinese-days 还没有收录 ${viewYear.value} 年的法定安排，这一年只按周末推算。`,
)

function weekdayLabel(date: Date): string {
  const dow = date.getDay()
  if (dow === 0) return '周日'
  if (dow === 6) return '周六'
  return `周${WEEKDAYS[(dow + 6) % 7]}`
}

function formatDay(date: Date): string {
  return `${date.getMonth() + 1} 月 ${date.getDate()} 日`
}

function isoWeek(date: Date): number {
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const dayNumber = (target.getDay() + 6) % 7
  target.setDate(target.getDate() - dayNumber + 3)
  const firstThursday = new Date(target.getFullYear(), 0, 4)
  const firstDayNumber = (firstThursday.getDay() + 6) % 7
  firstThursday.setDate(firstThursday.getDate() - firstDayNumber + 3)
  return 1 + Math.round((target.getTime() - firstThursday.getTime()) / (7 * 24 * 3600 * 1000))
}

function dayOfYear(date: Date): number {
  const start = new Date(date.getFullYear(), 0, 1)
  return Math.floor((date.getTime() - start.getTime()) / 86400000) + 1
}

function stepMonth(delta: number): void {
  const next = new Date(viewYear.value, viewMonth.value + delta, 1)
  viewYear.value = next.getFullYear()
  viewMonth.value = next.getMonth()
}

function goToday(): void {
  viewYear.value = today.getFullYear()
  viewMonth.value = today.getMonth()
}
</script>

<template>
  <div class="page calendar">
    <header class="page__head">
      <p class="eyebrow">日历</p>
      <h1 class="page__title">工作日与假期</h1>
      <p class="page__lead">
        放假日、调休和周末一眼看清，顺带看看农历节气，和离下一个休息日还有多远。
      </p>
    </header>

    <section class="cal">
      <header class="cal__bar">
        <div class="cal__nav">
          <button class="cal__step" type="button" title="上个月" @click="stepMonth(-1)">
            <PhCaretLeft :size="16" weight="bold" />
          </button>
          <span class="cal__month">{{ monthLabel }}</span>
          <button class="cal__step" type="button" title="下个月" @click="stepMonth(1)">
            <PhCaretRight :size="16" weight="bold" />
          </button>
        </div>

        <div class="cal__tools">
          <button class="btn btn--sm" type="button" @click="goToday">
            <PhCalendarBlank :size="15" />
            回到今天
          </button>
        </div>
      </header>

      <div class="cal__weekdays" aria-hidden="true">
        <span v-for="(label, i) in WEEKDAYS" :key="label" :class="{ 'is-weekend': i >= 5 }">
          {{ label }}
        </span>
      </div>

      <div class="cal__grid">
        <div
          v-for="cell in cells"
          :key="cell.key"
          class="day"
          :class="{
            'is-out': !cell.inMonth,
            'is-weekend': cell.isWeekend,
            'is-rest': cell.isRest && !cell.isWeekend,
            'is-adjust': cell.isAdjust,
            'is-today': cell.isToday,
          }"
          :title="`${cell.key}${cell.name ? ' · ' + cell.name : ''}`"
        >
          <span class="day__num">{{ cell.day }}</span>
          <span v-if="cell.isAdjust" class="day__badge">班</span>
          <span v-else-if="cell.isRest && !cell.isWeekend" class="day__badge day__badge--rest">
            休
          </span>
          <span v-if="cell.name" class="day__name">{{ cell.name }}</span>
        </div>
      </div>

      <p class="cal__note">{{ note }}</p>
    </section>

    <section class="cards">
      <article class="card">
        <p class="card__label">下一个休息日</p>
        <p class="card__value">{{ countdown }}</p>
        <p v-if="nextRest" class="card__foot">
          {{ formatDay(nextRest.date) }} · {{ weekdayLabel(nextRest.date) }}
          <template v-if="nextRest.name"> · {{ nextRest.name }}</template>
        </p>
        <p v-else class="card__foot">—</p>
      </article>

      <article class="card">
        <p class="card__label">本月</p>
        <p class="card__value">{{ monthStats.workdays }} 个工作日</p>
        <p class="card__foot">休息 {{ monthStats.rests }} 天 · 共 {{ monthStats.total }} 天</p>
      </article>

      <article class="card">
        <p class="card__label">今天</p>
        <p class="card__value">
          {{ todayInfo.rest ? '休息日' : '工作日' }}
          <span v-if="todayInfo.name" class="card__tag">{{ todayInfo.name }}</span>
        </p>
        <p class="card__foot">
          农历{{ todayInfo.lunar }} · 第 {{ todayInfo.week }} 周 · 今年第
          {{ todayInfo.dayNumber }} 天
        </p>
      </article>
    </section>

    <ul class="legend">
      <li class="legend__item"><span class="legend__dot legend__dot--today" />今天</li>
      <li class="legend__item"><span class="legend__dot legend__dot--rest" />法定放假</li>
      <li class="legend__item"><span class="legend__dot legend__dot--adjust" />调休上班</li>
      <li class="legend__item"><span class="legend__dot legend__dot--weekend" />周末</li>
      <li class="legend__item"><span class="legend__dot legend__dot--term" />节气 / 农历</li>
    </ul>
  </div>
</template>

<style scoped>
.cal {
  margin-top: var(--space-xl);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.cal__bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: var(--space-md) var(--space-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--bg-elevated) 70%, transparent);
}

.cal__nav {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.cal__month {
  min-width: 9ch;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-xl);
  text-align: center;
}

.cal__step {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-inset);
  color: var(--text-secondary);
  cursor: pointer;
  transition:
    color var(--dur-fast) var(--ease-out),
    border-color var(--dur-fast) var(--ease-out);
}

.cal__step:hover {
  border-color: var(--accent-border);
  color: var(--accent);
}

.cal__tools {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-xs);
}

.cal__weekdays {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  border-bottom: 1px solid var(--border-subtle);
}

.cal__weekdays span {
  padding: var(--space-sm) 0;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-align: center;
}

.cal__weekdays span.is-weekend {
  color: var(--accent);
}

.cal__grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
}

.day {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-height: 86px;
  padding: var(--space-sm);
  border-right: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  transition: background var(--dur-fast) var(--ease-out);
}

.day:nth-child(7n) {
  border-right: none;
}

.day:hover {
  background: var(--bg-hover);
}

.day__num {
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.day__name {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  line-height: 1.35;
}

.day__badge {
  position: absolute;
  top: var(--space-sm);
  right: var(--space-sm);
  min-width: 18px;
  padding: 1px 4px;
  border-radius: var(--radius-xs);
  background: var(--info);
  color: var(--text-inverse);
  font-size: var(--text-2xs);
  text-align: center;
}

.day__badge--rest {
  background: var(--danger);
}

.day.is-out {
  opacity: 0.35;
}

.day.is-weekend .day__num {
  color: var(--accent);
}

.day.is-rest {
  background: color-mix(in srgb, var(--danger) 8%, transparent);
}

.day.is-rest .day__num {
  color: var(--danger);
}

.day.is-adjust {
  background: color-mix(in srgb, var(--info) 9%, transparent);
}

.day.is-adjust .day__num {
  color: var(--info);
}

.day.is-today {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.day.is-today .day__num {
  color: var(--accent);
  font-weight: 600;
}

.cal__note {
  margin: 0;
  padding: var(--space-md) var(--space-lg);
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.cards {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-md);
  margin-top: var(--space-lg);
}

.card {
  padding: var(--space-lg);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.card__label {
  margin: 0 0 var(--space-xs);
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.card__value {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-2xl);
}

.card__tag {
  padding: 2px 8px;
  border-radius: var(--radius-full);
  background: var(--accent-subtle);
  color: var(--accent);
  font-family: var(--font-body);
  font-size: var(--text-2xs);
}

.card__foot {
  margin: var(--space-xs) 0 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-lg);
  margin: var(--space-lg) 0 0;
  padding: 0;
  list-style: none;
}

.legend__item {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs);
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.legend__dot {
  width: 10px;
  height: 10px;
  border-radius: var(--radius-full);
}

.legend__dot--today {
  border: 2px solid var(--accent);
}

.legend__dot--rest {
  background: var(--danger);
}

.legend__dot--adjust {
  background: var(--info);
}

.legend__dot--weekend {
  background: var(--accent);
}

.legend__dot--term {
  background: var(--text-muted);
}

@media (max-width: 900px) {
  .cards {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .day {
    min-height: 62px;
    padding: var(--space-2xs);
  }

  .day__num {
    font-size: var(--text-base);
  }

  .day__name {
    display: none;
  }

  .cal__month {
    font-size: var(--text-lg);
  }
}
</style>
