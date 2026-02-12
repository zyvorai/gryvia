# KubeFabric Web UI

Modern React-based web dashboard for KubeFabric GPU compute platform.

## Features

- **Real-time Dashboard** - Cluster stats, GPU utilization, and recent jobs
- **Job Management** - Submit, monitor, and manage AI training jobs
- **Quota Tracking** - Team-based GPU quotas and budget monitoring
- **Node Monitoring** - Real-time GPU metrics (temperature, utilization, memory)
- **Cost Analysis** - GPU compute costs by team and GPU type
- **Responsive Design** - TailwindCSS-based UI with mobile support

## Tech Stack

- **React 18** - Modern React with hooks
- **TypeScript** - Type-safe development
- **Vite** - Fast build tool and dev server
- **TailwindCSS** - Utility-first CSS framework
- **React Query** - Data fetching and caching
- **React Router** - Client-side routing
- **Recharts** - Data visualization
- **Lucide React** - Icon library

## Development

### Prerequisites

- Node.js 18+ and npm
- Running KubeFabric cluster with API accessible

### Installation

```bash
cd web-ui
npm install
```

### Development Server

```bash
npm run dev
```

The application will be available at `http://localhost:5173`

The Vite dev server is configured to proxy API requests to `/api` to `http://localhost:8001` (kube-apiserver proxy).

### Build for Production

```bash
npm run build
```

The production-ready files will be in the `dist/` directory.

### Preview Production Build

```bash
npm run preview
```

## Configuration

### API Proxy

The Vite config (`vite.config.ts`) proxies API requests to the Kubernetes API server:

```typescript
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8001',
      changeOrigin: true,
    },
  },
}
```

For production, you'll need to configure nginx or similar to proxy `/api` to the Kubernetes API server.

### Kubernetes API Access

The web UI accesses the Kubernetes API through the following endpoints:

- `GET /api/apis/kubefabric.io/v1/namespaces/default/fabricaijobs` - List jobs
- `GET /api/apis/kubefabric.io/v1/fabricquotas` - List quotas
- `GET /api/apis/kubefabric.io/v1/fabricgpunodes` - List GPU nodes
- `POST /api/apis/kubefabric.io/v1/namespaces/default/fabricaijobs` - Create job
- `DELETE /api/apis/kubefabric.io/v1/namespaces/default/fabricaijobs/{name}` - Delete job

## Deployment

### Docker

Build the Docker image:

```bash
docker build -t kubefabric-ui:latest -f docker/Dockerfile.ui .
```

Run the container:

```bash
docker run -p 8080:80 kubefabric-ui:latest
```

### Kubernetes

Deploy to Kubernetes:

```bash
kubectl apply -f manifests/deploy/ui-deployment.yaml
kubectl apply -f manifests/deploy/ui-service.yaml
kubectl apply -f manifests/deploy/ui-ingress.yaml
```

## Project Structure

```
web-ui/
├── src/
│   ├── components/       # Reusable UI components
│   │   ├── Layout.tsx
│   │   ├── StatCard.tsx
│   │   ├── JobsTable.tsx
│   │   ├── GPUChart.tsx
│   │   └── LoadingSpinner.tsx
│   ├── pages/           # Page components
│   │   ├── Dashboard.tsx
│   │   ├── Jobs.tsx
│   │   ├── JobDetails.tsx
│   │   ├── SubmitJob.tsx
│   │   ├── Quotas.tsx
│   │   ├── Nodes.tsx
│   │   └── Costs.tsx
│   ├── lib/             # Utilities and API client
│   │   └── api.ts
│   ├── types/           # TypeScript type definitions
│   │   └── index.ts
│   ├── App.tsx          # Main app component
│   ├── main.tsx         # Entry point
│   └── index.css        # Global styles
├── index.html           # HTML template
├── package.json
├── tsconfig.json
├── vite.config.ts
└── tailwind.config.js
```

## Environment Variables

None required - configuration is done through the Vite proxy config.

## License

Apache 2.0
