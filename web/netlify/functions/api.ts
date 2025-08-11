import { Handler } from '@netlify/functions';
import { OpenAPIBackend } from 'openapi-backend';
import { join } from 'path';

let specPath: string;

if (process.env.NETLIFY_LOCAL) {
  // Running in netlify dev (source files available)
  specPath = join(process.cwd(), '../api/docs/swagger.yaml');
} else {
  // Running in production (packaged functions dir)
  specPath = './api/docs/swagger.yaml'
}

// Initialize OpenAPI Backend with automatic mock generation
const api = new OpenAPIBackend({
  definition: specPath,
});

api.register({
  // Use default mock handler for all operations
  notFound: () => new Response('Not Found', { status: 404 }),
  validationFail: (c) => new Response(JSON.stringify({
    message: 'Validation failed',
    errors: c.validation.errors
  }), {
    status: 400,
    headers: { 'Content-Type': 'application/json' }
  }),
  // Default handler that generates mocks from OpenAPI examples
  notImplemented: (c) => {
    const { status, mock } = c.api.mockResponseForOperation(c.operation.operationId!);
    return new Response(JSON.stringify(mock), {
      status: status || 200,
      headers: { 'Content-Type': 'application/json' }
    });
  }
})

// Initialize the API
api.init();

export const handler: Handler = async (event, context) => {
  const { path, httpMethod, headers, body } = event;

  // Extract the API path (remove /.netlify/functions/api prefix)
  const apiPath = path.replace(/^\/api/, '') || '/';

  // Handle CORS preflight requests
  if (httpMethod === 'OPTIONS') {
    return {
      statusCode: 200,
      headers: {
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, PATCH, OPTIONS',
        'Access-Control-Allow-Headers': 'Content-Type, Authorization',
        'Access-Control-Max-Age': '86400'
      }
    };
  }

  try {
    // Handle the request with OpenAPI Backend
    const response = await api.handleRequest(
      {
        method: httpMethod,
        path: apiPath,
        query: event.queryStringParameters || {},
        body: body,
        headers: headers || {}
      }
    );

    return {
      statusCode: response.status,
      headers: {
        'Content-Type': 'application/json',
        'Access-Control-Allow-Origin': '*',
        ...Object.fromEntries(response.headers.entries())
      },
      body: await response.text()
    };
  } catch (error) {
    console.error('API Error:', error);
    return {
      statusCode: 500,
      headers: {
        'Content-Type': 'application/json',
        'Access-Control-Allow-Origin': '*'
      },
      body: JSON.stringify({ message: 'Internal Server Error' })
    };
  }
};
