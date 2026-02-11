# Web (Frontend)

Next.js app for Phase 1 with minimal login and dashboard placeholder.

## Tech stack
- Next.js 15
- React 19
- TypeScript
- Tailwind CSS
- Local package dependency on `@moveops/client`

## How it is built
- Uses `apps/web/Dockerfile` for production build.
- Builds `packages/client` first (types/client generation + TS build), then builds Next.js app.

## Environment
- `NEXT_PUBLIC_API_URL` default `http://localhost:8080/api`

## Start locally (dev)
From repo root:
```bash
cd packages/client
npm install
npm run gen
npm run build

cd ../../apps/web
npm install
npm run dev
```

App runs on `http://localhost:3000`.

## Start via Docker Compose
From repo root:
```bash
docker compose up --build
```

## How it works (high level)
- `/login` submits email/password to backend `POST /auth/login` with cookie auth.
- After login, frontend fetches `/auth/csrf` and stores token in session storage for future state-changing requests.
- `/` fetches `/auth/me` and shows basic current user + tenant info.
