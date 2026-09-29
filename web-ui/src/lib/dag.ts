// Layers a workflow's steps by dependency depth (topological levels).

type StepLike = { name: string; dependsOn?: string[] }

/**
 * Level 0 = steps with no (known) dependencies; every other step sits one level below its deepest
 * dependency. Order within a level follows the input order. Unknown dependencies are ignored and
 * dependency cycles do not loop forever (the cycle edge is dropped).
 */
export function dagLayers<T extends StepLike>(steps: T[]): T[][] {
  const byName = new Map(steps.map((s) => [s.name, s]))
  const memo = new Map<string, number>()
  const visiting = new Set<string>()

  const level = (name: string): number => {
    const cached = memo.get(name)
    if (cached !== undefined) return cached
    if (visiting.has(name)) return 0
    visiting.add(name)
    let depth = 0
    for (const dep of byName.get(name)?.dependsOn ?? []) {
      if (dep !== name && byName.has(dep)) depth = Math.max(depth, level(dep) + 1)
    }
    visiting.delete(name)
    memo.set(name, depth)
    return depth
  }

  const layers: T[][] = []
  for (const step of steps) {
    const l = level(step.name)
    ;(layers[l] ??= []).push(step)
  }
  return layers.filter(Boolean)
}
