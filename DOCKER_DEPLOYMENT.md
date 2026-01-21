# Docker Deployment Guide for BOPDS

## Prerequisites

- Docker 20.10+
- Docker Compose 2.0+

## Quick Start

### Production Deployment

1. **Prepare Environment**

   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

2. **Build the Image**

   ```bash
   docker build -t bopds:production .
   ```

3. **Start the Service**

   ```bash
   docker-compose up -d
   ```

4. **Initialize Database** (First time only)

   ```bash
   docker-compose exec bopds /app/bopds init
   ```

5. **Scan Library** (After adding books to `./lib`)

   ```bash
   docker-compose exec bopds /app/bopds scan
   ```

6. **Access the Application**
   - Open <http://docker-host:3001> in your browser

### Development Environment

1. **Start Development Container**

   ```bash
   docker-compose -f docker-compose.dev.yml up --build
   ```

2. **View Logs**

   ```bash
   docker-compose -f docker-compose.dev.yml logs -f
   ```

3. **Attach to Container** (For debugging)

   ```bash
   docker attach bopds-dev-server
   # Press Ctrl+P then Ctrl+Q to detach without stopping
   ```

4. **Hot Reload**
   - Go changes: Automatically reloaded by Air
   - Vue changes: Rebuild inside container:

     ```bash
     docker-compose -f docker-compose.dev.yml exec bopds-dev bash -c "cd frontend && npm run build"
     ```

## Volume Management

### Backup Database

```bash
docker run --rm \
  -v bopds_db-data:/data \
  -v $(pwd):/backup \
  debian:bookworm-slim \
  tar czf /backup/books.db.backup.gz -C /data books.db
```

### Restore Database

```bash
docker run --rm \
  -v bopds_db-data:/data \
  -v $(pwd):/backup \
  debian:bookworm-slim \
  tar xzf /backup/books.db.backup.gz -C /data
```

### Inspect Database Location

```bash
docker volume inspect bopds_db-data
```

## Production Tips

1. **Reverse Proxy** (Recommended)
   Use Nginx or Traefik for SSL termination and proper headers

2. **Resource Limits**
   Adjust CPU and memory limits in `docker-compose.yml` based on your library size

3. **Read-Only Root Filesystem**
   For enhanced security, uncomment `read_only: true` in docker-compose.yml

4. **Log Rotation**
   Log rotation is configured with max-size: "10m" and max-file: "3"

5. **Health Monitoring**
   The health check is available at `http://localhost:3001/`

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs bopds

# Check configuration
docker-compose config
```

### Database Locked

```bash
# Stop container
docker-compose down

# Check for WAL files
ls -la $(docker volume inspect bopds_db-data -f '{{.Mountpoint}}')

# Restart
docker-compose up -d
```

### CGO Errors

The Dockerfile uses Debian (not Alpine) for proper glibc support with CGO/SQLite

### Large Library Performance

For libraries with thousands of books, increase resource limits in docker-compose.yml

## Updating

1. **Rebuild Image**

   ```bash
   docker-compose build --no-cache
   ```

2. **Restart with Zero Downtime**

   ```bash
   docker-compose up -d --no-deps --build bopds
   ```

## Security Checklist

- [x] Run as non-root user
- [x] Use read-only library volume mount
- [x] Enable no-new-privileges
- [x] Set resource limits
- [x] Configure log rotation
- [ ] Use reverse proxy for SSL
- [ ] Regular image updates
- [ ] Scan images for vulnerabilities: `docker scan bopds:production`

## Build Targets

The Dockerfile supports multiple build targets:

- **Production** (default): `docker build -t bopds:production .`
- **Development**: `docker build --target development -t bopds:dev .`
- **Base**: `docker build --target base -t bopds:base .`
- **Frontend builder**: `docker build --target frontend-builder -t bopds:frontend .`
- **Backend builder**: `docker build --target backend-builder -t bopds:backend .`
