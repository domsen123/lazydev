// The JSON of "lazydev status --json": one entry per PRD of the project, open ones only.

export type StepState = 'done' | 'running' | 'failed' | 'pending'

export type Step = { name: string; state: StepState; cost?: number; startedAt?: string; endedAt?: string }

export type Item = {
  project: string
  prd: string
  label: string
  phase: string
  status?: string
  queue?: number
  step?: string
  port?: number
  cost?: number
  error?: string
  note?: string
  blocker?: { step: string; questions: string[] }
  waitsFor?: string[]
  afterReason?: string
  resolved?: number
  startedAt?: string
  model?: string
  steps?: Step[]
  devPid?: number
}

// Snapshot is the last read: the items, an error of the read, and when it was taken (ms).
export type Snapshot = { items: Item[]; error: string; at: number }

declare module 'claude-code' {
  interface PluginState {
    'lazydev-runs': { snapshot: Shaped<Snapshot> }
  }
}
