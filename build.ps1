# Build and Push Script for Era Inventory API
# Usage: .\build.ps1 [command]

param(
    [Parameter(Position=0)]
    [string]$Command = "help"
)

# Variables
$ImageName = "era-inventory-api"
$Registry = "ghcr.io"
$FullImageName = "$Registry/$ImageName"
$Version = git describe --tags --always --dirty 2>$null
if (-not $Version) { $Version = "latest" }

function Show-Help {
    Write-Host "Usage: .\build.ps1 [command]" -ForegroundColor Green
    Write-Host ""
    Write-Host "Commands:" -ForegroundColor Green
    Write-Host "  build              - Build the Go binary locally" -ForegroundColor Yellow
    Write-Host "  build-windows      - Build the Go binary for Windows" -ForegroundColor Yellow
    Write-Host "  test               - Run tests" -ForegroundColor Yellow
    Write-Host "  test-coverage      - Run tests with coverage" -ForegroundColor Yellow
    Write-Host "  clean              - Clean build artifacts" -ForegroundColor Yellow
    Write-Host "  docker-build       - Build Docker image" -ForegroundColor Yellow
    Write-Host "  docker-run         - Run Docker container locally" -ForegroundColor Yellow
    Write-Host "  docker-compose-up  - Start all services with Docker Compose" -ForegroundColor Yellow
    Write-Host "  docker-compose-down- Stop all services with Docker Compose" -ForegroundColor Yellow
    Write-Host "  docker-push        - Push Docker image to registry" -ForegroundColor Yellow
    Write-Host "  docker-push-ghcr   - Push to GitHub Container Registry" -ForegroundColor Yellow
    Write-Host "  docker-push-dockerhub - Push to Docker Hub" -ForegroundColor Yellow
    Write-Host "  lint               - Run linting" -ForegroundColor Yellow
    Write-Host "  fmt                - Format code" -ForegroundColor Yellow
    Write-Host "  mod-tidy           - Tidy Go modules" -ForegroundColor Yellow
    Write-Host "  security-scan      - Run security scan on Docker image" -ForegroundColor Yellow
    Write-Host "  all                - Run all: clean, test, build, and docker-build" -ForegroundColor Yellow
    Write-Host "  release            - Full release process" -ForegroundColor Yellow
}

function Invoke-Build {
    Write-Host "Building Go binary..." -ForegroundColor Green
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    go build -o bin/api.exe ./cmd/api
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build successful!" -ForegroundColor Green
    } else {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
}

function Invoke-BuildWindows {
    Write-Host "Building Go binary for Windows..." -ForegroundColor Green
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -o bin/api.exe ./cmd/api
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Windows build successful!" -ForegroundColor Green
    } else {
        Write-Host "Windows build failed!" -ForegroundColor Red
        exit 1
    }
}

function Invoke-Test {
    Write-Host "Running tests..." -ForegroundColor Green
    go test -v ./...
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Tests passed!" -ForegroundColor Green
    } else {
        Write-Host "Tests failed!" -ForegroundColor Red
        exit 1
    }
}

function Invoke-TestCoverage {
    Write-Host "Running tests with coverage..." -ForegroundColor Green
    go test -v -coverprofile=coverage.out ./...
    if ($LASTEXITCODE -eq 0) {
        go tool cover -html=coverage.out -o coverage.html
        Write-Host "Coverage report generated: coverage.html" -ForegroundColor Green
    } else {
        Write-Host "Tests failed!" -ForegroundColor Red
        exit 1
    }
}

function Invoke-Clean {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Green
    if (Test-Path "bin") { Remove-Item -Recurse -Force "bin" }
    if (Test-Path "coverage.out") { Remove-Item "coverage.out" }
    if (Test-Path "coverage.html") { Remove-Item "coverage.html" }
    Write-Host "Cleanup complete!" -ForegroundColor Green
}

function Invoke-DockerBuild {
    Write-Host "Building Docker image..." -ForegroundColor Green
    docker build -t "$ImageName`:$Version" .
    if ($LASTEXITCODE -eq 0) {
        docker tag "$ImageName`:$Version" "$ImageName`:latest"
        Write-Host "Docker build successful!" -ForegroundColor Green
        Write-Host "Image: $ImageName`:$Version" -ForegroundColor Cyan
    } else {
        Write-Host "Docker build failed!" -ForegroundColor Red
        exit 1
    }
}

function Invoke-DockerRun {
    Write-Host "Running Docker container..." -ForegroundColor Green
    docker run -p 8080:8080 --env-file .env "$ImageName`:latest"
}

function Invoke-DockerComposeUp {
    Write-Host "Starting Docker Compose services..." -ForegroundColor Green
    docker-compose up -d
}

function Invoke-DockerComposeDown {
    Write-Host "Stopping Docker Compose services..." -ForegroundColor Green
    docker-compose down
}

function Invoke-DockerPush {
    Write-Host "Pushing Docker image to registry..." -ForegroundColor Green
    docker tag "$ImageName`:$Version" "$FullImageName`:$Version"
    docker tag "$ImageName`:latest" "$FullImageName`:latest"
    docker push "$FullImageName`:$Version"
    docker push "$FullImageName`:latest"
    Write-Host "Push successful!" -ForegroundColor Green
}

function Invoke-DockerPushGHCR {
    Write-Host "Pushing to GitHub Container Registry..." -ForegroundColor Green
    Write-Host "Please ensure you have logged in with: docker login ghcr.io -u USERNAME -p TOKEN" -ForegroundColor Yellow
    docker tag "$ImageName`:$Version" "$FullImageName`:$Version"
    docker tag "$ImageName`:latest" "$FullImageName`:latest"
    docker push "$FullImageName`:$Version"
    docker push "$FullImageName`:latest"
    Write-Host "Push to GHCR successful!" -ForegroundColor Green
}

function Invoke-DockerPushDockerHub {
    Write-Host "Pushing to Docker Hub..." -ForegroundColor Green
    Write-Host "Please ensure you have logged in with: docker login" -ForegroundColor Yellow
    docker tag "$ImageName`:$Version" "$ImageName`:$Version"
    docker tag "$ImageName`:latest" "$ImageName`:latest"
    docker push "$ImageName`:$Version"
    docker push "$ImageName`:latest"
    Write-Host "Push to Docker Hub successful!" -ForegroundColor Green
}

function Invoke-Lint {
    Write-Host "Running linting..." -ForegroundColor Green
    golangci-lint run
}

function Invoke-Fmt {
    Write-Host "Formatting code..." -ForegroundColor Green
    go fmt ./...
}

function Invoke-ModTidy {
    Write-Host "Tidying Go modules..." -ForegroundColor Green
    go mod tidy
    go mod verify
}

function Invoke-SecurityScan {
    Write-Host "Running security scan..." -ForegroundColor Green
    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock `
        -v ${PWD}:/workspace `
        aquasec/trivy image "$ImageName`:$Version"
}

function Invoke-All {
    Write-Host "Running all: clean, test, build, and docker-build..." -ForegroundColor Green
    Invoke-Clean
    Invoke-Test
    Invoke-Build
    Invoke-DockerBuild
}

function Invoke-Release {
    Write-Host "Running full release process..." -ForegroundColor Green
    Invoke-Clean
    Invoke-Test
    Invoke-DockerBuild
    Invoke-DockerPush
}

# Main execution
switch ($Command.ToLower()) {
    "build" { Invoke-Build }
    "build-windows" { Invoke-BuildWindows }
    "test" { Invoke-Test }
    "test-coverage" { Invoke-TestCoverage }
    "clean" { Invoke-Clean }
    "docker-build" { Invoke-DockerBuild }
    "docker-run" { Invoke-DockerRun }
    "docker-compose-up" { Invoke-DockerComposeUp }
    "docker-compose-down" { Invoke-DockerComposeDown }
    "docker-push" { Invoke-DockerPush }
    "docker-push-ghcr" { Invoke-DockerPushGHCR }
    "docker-push-dockerhub" { Invoke-DockerPushDockerHub }
    "lint" { Invoke-Lint }
    "fmt" { Invoke-Fmt }
    "mod-tidy" { Invoke-ModTidy }
    "security-scan" { Invoke-SecurityScan }
    "all" { Invoke-All }
    "release" { Invoke-Release }
    default { Show-Help }
}
