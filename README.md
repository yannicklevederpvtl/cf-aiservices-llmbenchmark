# Cloud Foundry GenAI LLM Benchmark

A comprehensive benchmarking tool for evaluating Large Language Model (LLM) performance on Cloud Foundry with GenAI services. Compare multiple models side-by-side with real-time progress tracking, export results, and modern web interface.

## 🚀 Quick Start

Deploy to Cloud Foundry in 3 steps:

```bash
# 1. Clone and navigate
git clone <repository-url>
cd cf-aiservices-llmbenchmark

# 2. Check your available services and bind your services
cf services

# 3. Deploy with your services
cf push

# 4. Access your app
cf apps
# Open the URL shown in your browser
```

## ✨ Key Features

- **Multi-Model Comparison**: Benchmark 2 models simultaneously with side-by-side results
- **Real-time Progress**: Live updates via Server-Sent Events during benchmark execution
- **Cloud Foundry Integration**: Automatic discovery of GenAI services via VCAP_SERVICES
- **Performance Metrics**: Throughput, latency, Time to First Token (TTFT), and concurrency analysis
- **Export/Import**: Save results as JSON or CSV, import previous results
- **Modern UI**: Responsive Vue.js interface with dark/light theme toggle
- **Concurrent Testing**: Test multiple concurrency levels (1, 2, 4, 8, etc.)
- **Job Management**: Cancel running benchmarks, view system status

## 📸 Screenshots

*[Screenshots will be added here showing the main interface, benchmark results, and export functionality]*

## 🧪 Local Testing

### Prerequisites

- **Go 1.23+**: For the backend server
- **Node.js 18+**: For the frontend development
- **GenAI Services**: Access to Cloud Foundry GenAI services or local OpenAI-compatible API

### Backend Setup

```bash
# Navigate to backend directory
cd llmapibenchmark

# Install Go dependencies
go mod tidy

# Set environment variables for local testing
export API_KEY="your-api-key"
export BASE_URL="https://your-genai-endpoint.com"
export GIN_MODE="debug"
export LOG_LEVEL="debug"

# Run the server
go run cmd/server/main.go
```

The backend will start on `http://localhost:8080` with:
- API endpoints: `http://localhost:8080/api/*`
- Health check: `http://localhost:8080/api/health`

### Local Configuration

Create a `.env` file in the project root for local testing:

```bash
# Backend configuration
API_KEY=your-api-key-here
BASE_URL=https://your-genai-endpoint.com
GIN_MODE=debug
LOG_LEVEL=debug

# Frontend configuration (for client/.env)
VITE_API_BASE_URL=http://localhost:8080
```

### Testing with Cloud Foundry Services

If you have access to Cloud Foundry services locally:

```bash
# Set VCAP_SERVICES environment variable
export VCAP_SERVICES='{"genai":[{"instance_guid":"...","credentials":{"api_key":"...","api_base":"..."}}]}'

# Run backend
cd llmapibenchmark && go run cmd/server/main.go
```

### API Testing

Test the backend directly:

```bash
# Health check
curl http://localhost:8080/api/health

# List models (if VCAP_SERVICES is set)
curl http://localhost:8080/api/models

# Start a benchmark
curl -X POST http://localhost:8080/api/benchmark/async \
  -H "Content-Type: application/json" \
  -d '{
    "model1": {"id": "test-model", "name": "test-model", "baseUrl": "https://api.openai.com/v1"},
    "concurrencyLevels": [1, 2],
    "maxTokens": 50,
    "prompt": "Hello world"
  }'
```

### Troubleshooting Local Development

**Backend won't start:**
```bash
# Check Go version
go version

# Verify dependencies
go mod verify

# Check for port conflicts
lsof -i :8080
```


## 🛠 Deployment

```
**Note**: Check your available services first:
```bash
cf services
```

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `CORS_ORIGIN` | CORS allowed origins | `*` |
| `GIN_MODE` | Gin framework mode | `release` |

### Service Binding

The application automatically discovers GenAI services from VCAP_SERVICES. Supported service types:

- **Multi-model services**: `multi-model-v1025`
- **Single-model services**: `gpt-oss-120b`
- **Legacy services**: Any GenAI service with standard credentials

## 🔌 API Reference

### Key Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/models` | GET | List available models |
| `/api/benchmark/async` | POST | Start async benchmark |
| `/api/jobs/{id}/stream` | GET | SSE stream for job progress |
| `/api/jobs/{id}` | GET | Get job status |
| `/api/jobs/{id}/cancel` | POST | Cancel running job |
| `/api/system-status/stream` | GET | SSE stream for system status |

### Benchmark Request Format

```json
{
  "model1": {
    "id": "service-id|model-name",
    "name": "model-name",
    "provider": "GenAI on Tanzu Platform",
    "baseUrl": "https://genai-proxy.sys.tas.com/service"
  },
  "model2": { /* same structure */ },
  "concurrencyLevels": [1, 2, 4],
  "maxTokens": 100,
  "prompt": "Your benchmark prompt",
  "numWords": 500
}
```

## 🏗 Architecture

### System Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Vue.js UI     │    │   Go Backend    │    │  GenAI Services │
│                 │    │                 │    │                 │
│ • Model Select  │◄──►│ • HTTP API      │◄──►│ • Multi-model   │
│ • Benchmark UI  │    │ • SSE Streams   │    │ • Single-model  │
│ • Results View  │    │ • Job Manager   │    │ • Legacy        │
│ • Export/Import │    │ • VCAP Parser   │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Key Components

- **Frontend**: Vue.js 3 with Composition API, Tailwind CSS, Radix Vue components
- **Backend**: Go with Gin framework, structured logging, SSE for real-time updates
- **Job Management**: Asynchronous job execution with cancellation support
- **Service Discovery**: Automatic VCAP_SERVICES parsing for GenAI service binding
- **API Integration**: OpenAI-compatible API calls with proper URL handling

### Data Flow

1. **Service Discovery**: Parse VCAP_SERVICES → Extract models → Return to frontend
2. **Benchmark Execution**: Frontend request → Create job → SSE progress → Return results
3. **Real-time Updates**: Job progress → SSE broadcast → Frontend updates
4. **Result Export**: Benchmark results → JSON/CSV format → Download

## 🔧 Troubleshooting

### Common Issues

**Models not appearing in UI**
```bash
# Check service bindings
cf services
cf env cf-genai-benchmark

# Check logs for service discovery
cf logs cf-genai-benchmark --recent | grep "Discovered service"
```

**Benchmark fails with 404 errors**
```bash
# Check API key resolution
cf logs cf-genai-benchmark --recent | grep "Found API key"

# Verify service credentials
cf env cf-genai-benchmark | grep VCAP_SERVICES
```

**SSE connection issues**
```bash
# Check for connection errors in browser console
# Verify proxy configuration in vite.config.ts
```

**Frontend not loading**
```bash
# Rebuild frontend
cd client && npm run build
cf push cf-genai-benchmark
```

### Log Analysis

```bash
# View all logs
cf logs cf-genai-benchmark --recent

# Filter for specific issues
cf logs cf-genai-benchmark --recent | grep ERROR
cf logs cf-genai-benchmark --recent | grep "API key"
cf logs cf-genai-benchmark --recent | grep "benchmark"
```

## 🚀 Performance

### Benchmark Metrics

- **Generation Speed**: Tokens per second for response generation
- **Prompt Throughput**: Tokens per second for prompt processing  
- **Time to First Token (TTFT)**: Latency before first response token
- **Concurrency Analysis**: Performance across different concurrency levels

### Optimization Tips

- Use appropriate concurrency levels (start with 1, 2, 4)
- Set reasonable max tokens (100-500 for quick tests)
- Monitor system resources during benchmarks
- Use shorter prompts for faster iteration

## 📝 License

MIT License - see LICENSE file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📞 Support

For issues and questions:
- Check the troubleshooting section above
- Review Cloud Foundry logs
- Open an issue in the repository

---

**Built for Cloud Foundry with ❤️**
