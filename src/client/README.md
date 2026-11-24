## JSERD Monitoring

This is a static Next.js SPA served from PocketBase. It authenticates only against
the same-origin PocketBase `chief_editors` collection and reads the safe monitoring
projection at `/api/jserd/monitoring`. The browser never calls OJS.

Run the checks with pnpm:

```bash
pnpm lint
pnpm test
pnpm build
```

To publish the build to the server, run `pnpm publish:pb`. This script copies
`out/` to `../server/pb_public/`.
