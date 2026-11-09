<!-- BEGIN:nextjs-agent-rules -->
 
# Next.js: ALWAYS read docs before coding
 
Before any Next.js work, find and read the relevant doc in `node_modules/next/dist/docs/`. Your training data is outdated — the docs are the source of truth.
 
<!-- END:nextjs-agent-rules -->

<!-- BEGIN:reactaria-agent-rules -->
 
# React-Aria: ALWAYS read docs before coding
 
Before any React-Aria work, find and read the relevant doc in `https://react-aria.adobe.com/getting-started`. Your training data is outdated — the docs are the source of truth.
 
<!-- END:reactaria-agent-rules -->

# React-Aria: every interactive component must use it

Every in-house interactive component must use `react-aria-components` (or a
`react-aria` hook). Do not hand-build interactive behavior with raw DOM
elements when Aria has a matching component (for example, `Button`,
`Checkbox`, `TextField`, `ComboBox`, `ListBox`, `Dialog`, `Modal`).

An exception is allowed only for a documented, strong reason, for example:

- A layout element with no interactive behavior (a status line, a read-only
  indicator, a real navigation `<a>` link).
- A case where Aria has no matching component or pattern for the required
  behavior.

# Cognitive complexity: keep every function at 15 or under

`biome lint` enforces this cap on every commit (`biome.json`). If a function
you write or edit exceeds 15, split it into smaller functions. Do not raise
the cap in `biome.json`.

