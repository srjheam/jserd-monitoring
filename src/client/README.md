## JSERD editorial monitoring

This is a static Next.js SPA served from PocketBase. It authenticates only against
the same-origin PocketBase `_superusers` collection and reads the safe monitoring
projection at `/api/jserd/monitoring`. The browser never calls OJS.

Run the checks with pnpm:

```bash
pnpm lint
pnpm test
pnpm build
pnpm test:browser
```

`pocketbase` is the only added runtime dependency. It provides the authenticated
same-origin client and persistent local auth store. Vitest is a development-only
dependency used for pure queue, search, sorting, unknown-evidence, and archive
precedence checks.

The browser smoke uses development-only `@playwright/test` and
`@axe-core/playwright`. It starts an isolated fixture-mode PocketBase server and
static export, then checks authentication, queue transitions, persistence, focus,
network boundaries, and serious/critical accessibility issues.
On hosts missing Chromium's NSPR, NSS, or ALSA libraries, its harness extracts
those packages into its own temporary directory and removes that directory on exit.
