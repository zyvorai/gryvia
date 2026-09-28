# Gryvia Web UI

Dark-themed React dashboard for Gryvia GPU compute platform, inspired by hyper2kvm's design language.

## Features

- **Real-time Dashboard** - Cluster stats, GPU utilization charts, job pipeline view
- **Job Management** - Submit, monitor, and manage AI training/inference jobs
- **Quota Tracking** - Team-based GPU quotas and budget monitoring
- **Node Monitoring** - Real-time GPU metrics (temperature, utilization, memory)
- **Cost Analysis** - GPU compute costs by team and GPU type with projections
- **Dark Theme** - Slate-based dark design with gradient stat cards and glow effects
- **Responsive** - Top navbar layout with mobile hamburger menu
- **Error Boundary** - Graceful error handling with recovery
- **Auth Support** - Bearer token auth via `localStorage` (`gryvia_token`) or `VITE_API_TOKEN` env var
- **404 Catch-All** - Unknown routes display a styled 404 page

## Design System

- **Background**: Dark slate (`#0f172a`)
- **Cards**: `bg-slate-800/50` with `border-slate-700/50` and rounded-xl
- **Stat Cards**: Gradient backgrounds (`stat-card-blue`, `stat-card-green`, etc.)
- **Text**: `text-white` (headings), `text-slate-300` (body), `text-slate-400` (muted)
- **Accents**: Blue (`#3b82f6`), Green (`#22c55e`), Purple (`#a855f7`), Cyan (`#06b6d4`)
- **Font**: Inter (Google Fonts)
- **Animations**: `animate-fade-in`, `animate-pulse-dot`, `card-glow` hover effects

## Tech Stack

- **React 18** with TypeScript
- **Vite** - Build tool and dev server
- **TailwindCSS** - Dark theme with CSS custom properties
- **React Query** - Data fetching with 10-30s refetch intervals
- **React Router** - Client-side routing
- **Recharts** - Charts with dark tooltip styling
- **Lucide React** - Icon library
- **Axios** - HTTP client with auth interceptors

## Development

```bash
cd web-ui
npm install
npm run dev     # http://localhost:5173
npm run build   # Production build to dist/
npm run lint    # ESLint check
```

## API

The UI communicates through the Gryvia API Gateway (not directly to the Kubernetes API):

| Endpoint | Description |
|----------|-------------|
| `GET /api/cluster/stats` | Cluster overview (GPUs, jobs, nodes) |
| `GET /api/jobs` | List AI jobs (pagination: `limit`, `offset`) |
| `GET /api/jobs/:name` | Get job details |
| `POST /api/jobs` | Submit new job |
| `DELETE /api/jobs/:name` | Delete job |
| `GET /api/quotas` | List team quotas (pagination: `limit`, `offset`) |
| `GET /api/quotas/:name` | Get quota details |
| `GET /api/nodes` | List GPU nodes (pagination: `limit`, `offset`) |
| `GET /api/nodes/:name` | Get node details |
| `GET /api/nodes/health` | Node health status |
| `GET /api/metrics/gpu` | GPU utilization metrics |
| `GET /api/metrics/costs` | Cost analysis data |
| `GET /api/metrics/jobs` | Job metrics |
| `GET /api/quota/usage` | Quota usage summary |

All endpoints require `Authorization: Bearer <token>` header.

Typed API responses include `ClusterStats`, `GPUMetricsResponse`, and `CostData` interfaces (see `src/lib/api.ts`).

## Deployment

```bash
# Build Docker image
docker build -t gryvia-ui:1.0.0 -f docker/Dockerfile.ui .

# Deploy to Kubernetes
kubectl apply -f manifests/deploy/ui-deployment.yaml
```

## Project Structure

```
web-ui/
  src/
    components/
      Layout.tsx          # Top navbar, mobile menu
      StatCard.tsx         # Gradient stat cards
      JobsTable.tsx        # Dark-themed job table
      GPUChart.tsx         # GPU utilization line chart
      LoadingSpinner.tsx   # Spinner with text
      ErrorBoundary.tsx    # React error boundary
    pages/
      Dashboard.tsx        # Main dashboard with pipeline
      Jobs.tsx             # Job listing + stats
      JobDetails.tsx       # Job detail view
      SubmitJob.tsx        # Job submission form
      Quotas.tsx           # Team quota cards
      Nodes.tsx            # GPU node metrics
      Costs.tsx            # Cost analysis charts
    lib/api.ts             # Typed API client with auth
    types/index.ts         # TypeScript types
    index.css              # Dark theme CSS (matches hyper2kvm)
    App.tsx                # Routes + error boundary
    main.tsx               # Entry point
```

## License

Apache 2.0
