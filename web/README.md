# Web Frontend

The web frontend for the Embedded Cluster Manager Experience - a React/TypeScript application built with Vite that provides the installation wizard interface for Embedded Cluster deployments.

## Quick Start

### Development with Mock Server

Run the development server with a mock API backend:

```bash
npm install
npm run dev
```

This starts:
- Vite dev server on `http://localhost:5173`
- Netlify Functions emulation for API mocking
- Hot module replacement for fast development

The mock server automatically generates responses from the OpenAPI specification at `../api/docs/swagger.yaml` using OpenAPI Backend.

### Building

```bash
npm run build
```

Builds the application for production to the `dist/` directory.

## Architecture

### Technology Stack

- **React** with functional components and hooks
- **TypeScript** for type safety
- **Vite** for fast development and building
- **Tailwind CSS** for styling
- **TanStack Query** for API state management
- **React Router** for navigation
- **Vitest** + React Testing Library for testing

### Project Structure

```
src/
├── components/           # Reusable UI components
│   ├── common/          # Shared components (Button, Input, Modal, etc.)
│   └── wizard/          # Installation wizard components
├── contexts/            # React contexts for global state
├── providers/           # Context providers
├── types/               # TypeScript type definitions
├── utils/               # Utility functions
└── test/                # Test setup and utilities
```

### Template Processing

The `index.html` file uses Go template syntax that requires pre-processing before serving:

- **Development**: Vite plugin handles template replacement with mock values
- **Production**: Go server processes templates with real configuration data

## API Integration

### TypeScript Type Generation

The web frontend uses TypeScript types generated from the OpenAPI specification located at `../api/docs/swagger.yaml`. These types ensure type safety when communicating with the Embedded Cluster API.

**Generating Types:**

Types are automatically generated before development and build:
- `npm run dev` (via `predev` script)
- `npm run build` (via `prebuild` script)

**Manual Generation:**

To manually regenerate types after API changes:

```bash
# From project root (recommended - generates OpenAPI docs + types)
make api-types

# Or from web directory (types only)
npm run types:api:generate
```

**Type Checking:**

To verify types are up-to-date without regenerating:

```bash
npm run types:api:check
```

The generated types are located at `src/types/api.ts` and used throughout the application with the `openapi-fetch` client.

### Mock Development Server

For local development, the application uses Netlify Functions to provide a mock API server:

- **Mock Generation**: Automatically generates mock responses from OpenAPI spec (`../api/docs/swagger.yaml`)
- **CORS Handling**: Built-in CORS support for local development
- **Real API Matching**: Mock responses match the production API contract
- **Hot Reloading**: Changes to the OpenAPI spec are reflected immediately

### Production API

In production, the web application communicates with the Embedded Cluster API server running on the same host, typically on port 30000.

## Netlify Integration

### Deploy Previews

The project is configured for Netlify deploy previews with automatic PR deployments:

- **Automatic Deployments**: Every PR gets a unique preview URL
- **Mock API Integration**: Deploy previews include full API mocking
- **OpenAPI Sync**: API spec is bundled with deployments for accurate mocking

## Testing

Run unit tests with:

```bash
npm run test:unit
```

## Linting

```bash
npm run lint          # Check for issues
npm run lint-fix      # Auto-fix issues
```
