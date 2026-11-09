// Run with: make test-mod. The file is not shipped with the mod.
import { expect, test } from 'claude-code/testing'

const PANE = {
  plugin: 'lazydev-runs',
  component: 'Pane',
  requestId: 'lazydev-runs',
  props: {
    title: 'lazydev',
    isFocused: false,
    bodyColumns: 70,
    placement: 'dock',
    scroll: { offset: 0, bodyRows: 40 },
    view: {},
  },
} as const

const steps = (done: number, running: boolean) =>
  ['plan', 'setup', 'implement', 'review', 'merge'].map((name, i) => ({
    name,
    state: i < done ? 'done' : i === done && running ? 'running' : 'pending',
  }))

const ITEMS = [
  { project: 'shop', prd: 'token', label: 'auth/token', phase: 'implement', step: 'implement', port: 3500, cost: 0.42,
    model: 'claude-opus-5-5', startedAt: '2026-11-09T10:00:00Z', steps: steps(2, true) },
  { project: 'shop', prd: 'schema', label: 'db/schema', phase: 'blocked', step: 'implement', cost: 0.2,
    blocker: { step: 'implement', questions: ['Which table holds the tokens?'] } },
  { project: 'shop', prd: 'login', label: 'auth/login', phase: 'waiting', queue: 1, waitsFor: ['token'],
    afterReason: 'both change the token store' },
  { project: 'shop', prd: 'docs', label: 'docs/readme', phase: 'planned' },
]

test('the pane shows running, blocked, waiting and planned PRDs on every surface', async ($, on) => {
  on('command.register', () => ({ value: { command: 'lazydev-runs' } }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('ui.panes', () => ({ value: [] }))
  on('clock.now', () => ({ value: Date.parse('2026-11-09T10:12:03Z') }))
  on('clock.every', () => ({ value: { cancel: () => {} } }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: JSON.stringify(ITEMS), stderr: '' } }))
  on('session.start', (_, e) => ({ cwd: e.cwd }))

  await $.session.start({ cwd: '/shop', surface: 'terminal', isInteractive: true })
  await $.command.run({ command: 'lazydev-runs', args: '' })

  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount({ ...PANE, surface })
    for (const text of [/Running · 1/, /auth\/token/, /2\/5 · implement/, /opus-5-5/, /Needs you · 1/,
      /Which table holds the tokens\?/, /Waiting · 1/, /after token/, /Planned · 1/, /≈\$0\.62/]) {
      expect(await ui.find({ type: 'Text', text })).toBeDefined()
    }
    await ui.unmount()
  }
})

test('a missing lazydev shows as an error', async ($, on) => {
  on('command.register', () => ({ value: { command: 'lazydev-runs' } }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('ui.panes', () => ({ value: [] }))
  on('clock.now', () => ({ value: 0 }))
  on('clock.every', () => ({ value: { cancel: () => {} } }))
  on('process.run', () => {
    throw new Error('not found')
  })
  on('session.start', (_, e) => ({ cwd: e.cwd }))

  await $.session.start({ cwd: '/', surface: 'terminal', isInteractive: true })
  await $.command.run({ command: 'lazydev-runs', args: '' })
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect(await ui.find({ type: 'Text', text: /lazydev not found in PATH/ })).toBeDefined()
  await ui.unmount()
})
