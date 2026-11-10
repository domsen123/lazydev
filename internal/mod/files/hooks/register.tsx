// lazydev-runs: a pane with the PRD runs of lazydev, like the agents panel. It reads
// "lazydev status --here --json" in the session's folder (only the project of that folder, nothing
// outside one) every 3 s while the pane is open, every 15 s while it is closed, and opens the pane
// by itself once a run works, waits or is blocked, unless the person closed it this session.
// /lazydev-runs opens and closes it.
import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { Item, Snapshot, Step } from '../types'

const PANE = 'lazydev-runs'
const TITLE = 'lazydev'
const TICK_MS = 3000
// While the pane is closed only every fifth tick reads.
const CLOSED_EVERY = 5

// Change the tag when Snapshot changes: it declines what an earlier version stored.
const snapshot = atom({ plugin: 'lazydev-runs', key: 'snapshot' } as const, { items: [], error: '', at: 0 } as Snapshot, {
  shape: 'snapshot-1',
})

const RUNNING = ['plan', 'setup', 'implement', 'resolve', 'review', 'merge-queue', 'merge']
const ATTENTION = ['blocked', 'error', 'aborted']

const SYMBOLS: Record<string, string> = {
  planned: '○', waiting: '◌', 'merge-queue': '◐', merged: '✓', aborted: '■', error: '✗', discarded: '✗', blocked: '⊘',
}

function symbol(phase: string): string {
  return SYMBOLS[phase] ?? (RUNNING.includes(phase) ? '●' : '?')
}

// color is a theme key per phase, so the pane follows the theme of Claude Code.
function color(phase: string): string {
  if (phase === 'merge-queue') return 'warning'
  if (phase === 'resolve') return 'claude'
  if (RUNNING.includes(phase)) return 'suggestion'
  if (phase === 'blocked') return 'warning'
  if (phase === 'error' || phase === 'aborted') return 'error'
  if (phase === 'merged') return 'success'
  return 'inactive'
}

function usd(n: number): string {
  return '$' + n.toFixed(2)
}

// duration is "m:ss" or "h:mm:ss" from start to now.
function duration(start: string | undefined, now: number): string {
  if (!start) return ''
  const s = Math.max(0, Math.floor((now - Date.parse(start)) / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = String(s % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`
}

function model(m: string | undefined): string {
  return m ? m.replace(/^claude-/, '') : ''
}

// progress is the share of done steps, half a step for the running one.
function progress(steps: Step[]): { done: number; total: number; share: number } {
  const done = steps.filter(s => s.state === 'done').length
  const running = steps.some(s => s.state === 'running') ? 0.5 : 0
  return { done, total: steps.length, share: steps.length ? (done + running) / steps.length : 0 }
}

function bar(width: number, share: number): [string, string] {
  const full = Math.round(Math.max(0, Math.min(1, share)) * width)
  return ['━'.repeat(full), '━'.repeat(Math.max(0, width - full))]
}

function cut(text: string, width: number): string {
  return text.length <= width ? text : text.slice(0, Math.max(0, width - 1)) + '…'
}

async function poll($: EngineInterface): Promise<Snapshot> {
  const at = await $.clock.now()
  try {
    const run = await $.process.run(['lazydev', 'status', '--here', '--json'], { timeoutMs: 15000 })
    if (run.exitCode !== 0) {
      return { items: [], error: (run.stderr || run.stdout).trim() || `lazydev status ended with ${run.exitCode}`, at }
    }
    return { items: JSON.parse(run.stdout) as Item[], error: '', at }
  } catch (err) {
    return { items: [], error: 'lazydev not found in PATH: ' + String(err), at }
  }
}

async function isOpen($: EngineInterface): Promise<boolean> {
  return (await $.ui.panes()).some(p => p.id === PANE)
}

async function refresh($: EngineInterface): Promise<Snapshot> {
  const snap = await poll($)
  await update($, snapshot, () => snap)
  return snap
}

function busy(items: Item[]): boolean {
  return items.some(it => RUNNING.includes(it.phase) || it.phase === 'waiting' || ATTENTION.includes(it.phase))
}

export const register: Register = on => {
  // closedByPerson keeps the pane closed after the person closed it, until /lazydev-runs.
  let closedByPerson = false
  let tick: { cancel: () => void } | undefined

  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.command.register({ name: 'lazydev-runs', description: 'Show or hide the lazydev runs of this project' })
    tick?.cancel()
    let n = 0
    const step = async () => {
      const open = await isOpen($)
      if (!open && n++ % CLOSED_EVERY !== 0) return
      const snap = await refresh($)
      if (!open && !closedByPerson && busy(snap.items)) {
        await $.ui.open({ id: PANE, title: TITLE })
      }
    }
    void step()
    tick = $.clock.every(TICK_MS, () => void step())
    return started
  })

  on('command.run', { command: 'lazydev-runs' }, async $ => {
    if (await isOpen($)) {
      await $.ui.close({ id: PANE })
      closedByPerson = true
      return { text: 'lazydev pane closed.' }
    }
    closedByPerson = false
    await refresh($)
    await $.ui.open({ id: PANE, title: TITLE })
    return { text: 'lazydev pane opened.' }
  })

  on('ui.close', async ($, e, next) => {
    if (e.id === PANE && e.origin.kind === 'person') closedByPerson = true
    return next(e)
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text } = $.ui.resolve(e)
    const snap = await read($, snapshot)
    const width = Math.max(30, (e.viewport?.columns ?? 60) - 2)
    const items = snap.items
    const many = new Set(items.map(it => it.project)).size > 1
    const name = (it: Item) => (many ? it.project + '/' : '') + it.label

    if (snap.error) {
      return (
        <Box flexDirection="column">
          <Text color="error">{snap.error}</Text>
        </Box>
      )
    }

    const running = items.filter(it => RUNNING.includes(it.phase))
    const attention = items.filter(it => ATTENTION.includes(it.phase))
    const waiting = items.filter(it => it.phase === 'waiting').sort((a, b) => (a.queue ?? 0) - (b.queue ?? 0))
    const planned = items.filter(it => it.phase === 'planned')
    const cost = items.reduce((sum, it) => sum + (RUNNING.includes(it.phase) || ATTENTION.includes(it.phase) ? (it.cost ?? 0) : 0), 0)

    const card = (label: string, value: string, tone?: string) => (
      <Box borderStyle="round" borderDimColor flexDirection="column" paddingX={1} flexGrow={1}>
        <Text dimColor>{label}</Text>
        <Text bold color={tone}>{value}</Text>
      </Box>
    )

    const heading = (label: string, n: number) => (
      <Box marginTop={1}>
        <Text dimColor>
          {label} · {n}
        </Text>
      </Box>
    )

    const runRow = (it: Item) => {
      const steps = it.steps ?? []
      const p = progress(steps)
      const barWidth = Math.max(8, Math.min(40, width - 30))
      const [full, rest] = bar(barWidth, p.share)
      const info = [it.phase, model(it.model), it.port ? 'port ' + it.port + (it.devPid ? ' dev' : '') : '']
        .filter(Boolean)
        .join(' · ')
      return (
        <Box flexDirection="column" marginTop={1}>
          <Text bold>
            <Text color={color(it.phase)}>{symbol(it.phase)} </Text>
            {cut(name(it), width - 2)}
          </Text>
          <Text>
            {'  '}
            <Text color={color(it.phase)}>{info}</Text>
            {it.resolved ? <Text dimColor> · unblocked by Claude</Text> : ''}
          </Text>
          <Text>
            {'  '}
            {p.total ? `${p.done}/${p.total} · ` : ''}
            {it.step ?? it.phase}
            {'  '}
            <Text color={color(it.phase)}>{full}</Text>
            <Text dimColor>{rest}</Text>
            <Text dimColor>
              {'  '}
              {usd(it.cost ?? 0)} {duration(it.startedAt, snap.at)}
            </Text>
          </Text>
          {it.note ? <Text color="warning">{'  ' + cut(it.note, width - 2)}</Text> : ''}
        </Box>
      )
    }

    const attentionRow = (it: Item) => (
      <Box flexDirection="column" marginTop={1}>
        <Text bold>
          <Text color={color(it.phase)}>{symbol(it.phase)} </Text>
          {cut(name(it), width - 2)}
        </Text>
        <Text color={color(it.phase)}>
          {'  '}
          {it.phase === 'blocked' ? 'blocked in ' + (it.blocker?.step ?? it.step) : it.phase + (it.step ? ' in ' + it.step : '')}
          <Text dimColor> · {usd(it.cost ?? 0)}</Text>
        </Text>
        {(it.blocker?.questions ?? []).map(q => (
          <Text>{'  - ' + cut(q, width - 4)}</Text>
        ))}
        {it.phase !== 'blocked' && it.error ? <Text dimColor>{'  ' + cut(it.error, width - 2)}</Text> : ''}
      </Box>
    )

    const waitingRow = (it: Item) => (
      <Box flexDirection="column">
        <Text>
          <Text color="inactive">{symbol(it.phase)} </Text>
          {cut(name(it), width - 14)}
          <Text dimColor>
            {it.queue ? '  #' + it.queue : ''}
            {it.waitsFor?.length ? '  after ' + it.waitsFor.join(', ') : ''}
          </Text>
        </Text>
        {it.waitsFor?.length && it.afterReason ? <Text dimColor>{'  ' + cut(it.afterReason, width - 2)}</Text> : ''}
      </Box>
    )

    return (
      <Box flexDirection="column">
        <Box flexDirection="row" gap={1}>
          {card('Cost', '≈' + usd(cost))}
          {card('Running', String(running.length), running.length ? 'suggestion' : undefined)}
          {card('Waiting', String(waiting.length))}
          {attention.length ? card('Needs you', String(attention.length), 'warning') : ''}
        </Box>
        {items.length === 0 && (
          <Box marginTop={1}>
            <Text dimColor>No open PRDs. Write one with "make this a lazydev PRD".</Text>
          </Box>
        )}
        {running.length > 0 && heading('Running', running.length)}
        {running.map(runRow)}
        {attention.length > 0 && heading('Needs you', attention.length)}
        {attention.map(attentionRow)}
        {waiting.length > 0 && heading('Waiting', waiting.length)}
        {waiting.map(waitingRow)}
        {planned.length > 0 && heading('Planned', planned.length)}
        {planned.map(it => (
          <Text dimColor>
            {symbol(it.phase)} {cut(name(it), width - 2)}
          </Text>
        ))}
      </Box>
    )
  })
}
