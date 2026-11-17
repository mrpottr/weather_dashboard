# ATMOS — Australian Weather Intelligence Dashboard

ATMOS is an Australian weather intelligence dashboard combining a React + Vite frontend with a Go (Gin) + SQLite backend. It fetches weather data from Open-Meteo (using the BOM ACCESS-G feed) with graceful fallback, caches intelligently, computes accuracy against recent observations, delivers outdoor activity intelligence and surfaces BOM-style warnings/announcements via a clean, WCAG AA-friendly dark/light interface.

## Quick start

### Prerequisites
- Node.js 18+ / npm 9+ (Node 24 used during development)
- Go 1.20+ (Go 1.27 used)
- SQLite (included via modernc.org/sqlite, no external SQLite install required)

### 1. Install frontend dependencies
```bash
cd frontend
npm install
```

### 2. Start the backend
From a terminal (PowerShell shown):
```powershell
cd backend
$env:DB_PATH="..\database\atmos.db"
$env:DATA_DIR=".\data"
$env:JWT_SECRET="dev-secret-change-me"
go run .
```
The API listens on http://localhost:8080. On first start the SQLite database is created/seeded in `database/atmos.db` and the `backend/data/` directory is populated for CSV/JSON exports.

### 3. Start the frontend (development)
```bash
cd frontend
npm run dev
```
Vite serves on http://localhost:5173 by default with `/api` proxied to :8080.

### 4. Production build
```bash
cd frontend
npm run build
npm run preview
```

## Environment variables (backend)
| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `JWT_SECRET` | `dev-secret-change-me` | JWT signing key (change in production) |
| `DB_PATH` | `database/atmos.db` | Path to SQLite DB (relative or absolute) |
| `DATA_DIR` | `backend/data` | Directory for exported CSV/JSON files |
| `CORS_ORIGIN` | `*` | Allowed CORS origins |
| `PEXELS_API_KEY` | _(unset)_ | Photo provider fallback (optional) |
| `UNSPLASH_API_KEY` | _(unset)_ | Photo provider fallback (optional) |

## Default accounts
| Email | Password | Role | Notes |
| --- | --- | --- | --- |
| `admin@weather.local` | `admin123` | Administrator | Full access to Admin + User management |
| `user@weather.local` | `user123` | Standard user | Melbourne, VIC default; favourites seeded |

**Important:** change default credentials before deploying to production.

## Project structure
- `backend/` — Go (Gin) API; 11 files in package `main` (main, models, database, auth, weather_bom, locations, intelligence, history, export, admin, images). Exports written to `backend/data/`. Database at `database/atmos.db`.
- `frontend/` — React + Vite SPA with 1 entry (`index.html`), 1 API/state layer (`src/api/client.jsx`), 1 stylesheet (`src/styles.css`), 5 components, 13 pages (including admin/users). Uses react-router-dom v7, Leaflet (no react-leaflet) for maps.
- `database/` — SQLite database file (created at runtime).

## Data & source behaviour
- Primary source: BOM ACCESS-G via Open-Meteo (`/v1/bom`). If null/invalid data is returned, the backend falls back to Open-Meteo best-match (`/v1/forecast`) with `source_degraded=true` and `source/source_url` reflecting the actual serving endpoint. UI never misrepresents BOM feed health.
- All external weather calls occur server-side only; browser never calls third-party weather APIs directly.
- Caching, accuracy sampling (forecast-vs-latest observation), history retention/pruning and concurrent vs sequential retrieval are implemented per specification.

## Build & quality checks
Backend: `go fmt ./...`, `go vet ./...`, `go build ./...` must pass.
Frontend: `npm run build` must pass (Vite). Frontend lint was omitted to satisfy minimum-files rule.

## Notes
- Photo provider order: PEXELS → UNSPLASH → Wikimedia Commons fallback (no API keys set = Wikimedia by default).
- WCAG AA contrast targets met in both dark and light themes.
- No stubs or synthetic data; real BOM/Open-Meteo data only.