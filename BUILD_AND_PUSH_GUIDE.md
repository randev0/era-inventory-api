# Build and Push Guide for Era Inventory API

## Overview
This guide covers how to build, test, and push the Era Inventory API Docker image using the provided automation tools.

## Prerequisites
- Go 1.23+ installed
- Docker Desktop running
- PowerShell (for Windows users)

## Quick Start

### 1. Test the Build Process
```powershell
# Run the complete pipeline
.\build.ps1 all

# Or run individual steps
.\build.ps1 test
.\build.ps1 docker-build
```

### 2. Test the Application
```powershell
# Start all services
.\build.ps1 docker-compose-up

# Check if API is running
curl http://localhost:8080/health

# Stop services
.\build.ps1 docker-compose-down
```

### 3. Push to Container Registry

#### GitHub Container Registry (GHCR)
```powershell
# First, login to GHCR
docker login ghcr.io -u YOUR_USERNAME -p YOUR_TOKEN

# Then push
.\build.ps1 docker-push-ghcr
```

#### Docker Hub
```powershell
# First, login to Docker Hub
docker login

# Then push
.\build.ps1 docker-push-dockerhub
```

## Available Commands

### Build Commands
- `.\build.ps1 build` - Build Go binary for Linux
- `.\build.ps1 build-windows` - Build Go binary for Windows
- `.\build.ps1 docker-build` - Build Docker image

### Test Commands
- `.\build.ps1 test` - Run Go tests
- `.\build.ps1 test-coverage` - Run tests with coverage report

### Docker Commands
- `.\build.ps1 docker-run` - Run Docker container locally
- `.\build.ps1 docker-compose-up` - Start all services
- `.\build.ps1 docker-compose-down` - Stop all services
- `.\build.ps1 docker-compose-logs` - Show service logs

### Push Commands
- `.\build.ps1 docker-push` - Push to configured registry
- `.\build.ps1 docker-push-ghcr` - Push to GitHub Container Registry
- `.\build.ps1 docker-push-dockerhub` - Push to Docker Hub

### Utility Commands
- `.\build.ps1 clean` - Clean build artifacts
- `.\build.ps1 fmt` - Format Go code
- `.\build.ps1 mod-tidy` - Tidy Go modules
- `.\build.ps1 security-scan` - Run security scan on Docker image

### Pipeline Commands
- `.\build.ps1 all` - Run complete pipeline (clean, test, build, docker-build)
- `.\build.ps1 release` - Full release process (clean, test, build, docker-build, push)

## CI/CD Pipeline

The project includes a GitHub Actions workflow (`.github/workflows/build-and-push.yml`) that automatically:

1. **Tests** the code on every push and pull request
2. **Builds and pushes** Docker images on pushes to main/develop branches
3. **Scans** for security vulnerabilities
4. **Caches** dependencies for faster builds

## Docker Image Details

- **Base Image**: `golang:1.23-bullseye` (build stage)
- **Runtime Image**: `gcr.io/distroless/base-debian12` (minimal runtime)
- **Port**: 8080
- **Architecture**: Linux AMD64
- **Size**: ~55MB

## Environment Variables

Create a `.env` file based on `env.example`:
```bash
DB_DSN=postgres://postgres:postgres@localhost:5432/era?sslmode=disable
```

## Troubleshooting

### Common Issues

1. **Docker not running**: Ensure Docker Desktop is started
2. **Port conflicts**: Check if port 8080 is available
3. **Database connection**: Ensure PostgreSQL is running and accessible
4. **Permission errors**: Run PowerShell as Administrator if needed

### Debug Commands

```powershell
# Check Docker status
docker ps

# View Docker logs
.\build.ps1 docker-compose-logs

# Check image details
docker inspect era-inventory-api:latest

# Run container interactively
docker run -it --rm era-inventory-api:latest /bin/sh
```

## Security

- The Docker image uses distroless base for minimal attack surface
- Security scanning is performed with Trivy
- Dependencies are regularly updated
- Multi-stage build reduces final image size

## Performance

- Build caching is enabled for faster rebuilds
- Multi-stage Docker build optimizes image size
- Go modules are cached in CI/CD pipeline
- Docker layer caching reduces build time

## Next Steps

1. Set up your container registry credentials
2. Configure environment variables
3. Test the complete pipeline
4. Set up automated deployments
5. Monitor security scans and updates
