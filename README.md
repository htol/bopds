# BOPDS - Basic OPDS Server  (WORK IN PROGRESS)

A simple, performant web server for serving and managing FB2 (FictionBook 2.0) eBooks. Built with Go, SQLite, and Vue 3.

## Features

- **OPDS Protocol Support**: Full OPDS 1.2 feed implementation for eBook readers
- **Multi-format Books**: Support for FB2 files in ZIP and 7z archives
- **On-the-fly Conversion**: Convert FB2 to EPUB and MOBI formats on demand
- **Full-text Search**: Fast book search using SQLite FTS5 full-text search
- **Genre Classification**: Filter and browse books by genre
- **Web Interface**: Modern, responsive Vue 3 frontend with Tailwind CSS
- **Multiple Libraries**: Serve several libraries from one server; each root's first-level subdirectories are a library (see [Library Structure](#library-structure))
- **Library Scanning**: Metadata from `.inpx` indexes and loose `.inp` files, with rescan support (upsert by natural key, tombstoning of removed rows)
- **RESTful API**: Clean API for integration with other tools
- **Docker Support**: Easy deployment with Docker and Docker Compose

### Quick Start

TBD

## Configuration

Configuration is handled via environment variables. Create a `.env` file or set these directly:

```bash
# Server
PORT=3001
LOG_LEVEL=info          # debug, info, warn, error

# Database
DB_PATH=./books.db
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=25
DB_CONN_MAX_LIFETIME=300
DB_BATCH_SIZE=1000

# Library roots: PATH-style list; first-level subdirectories are libraries
LIBRARY_PATH=./lib
# Display names (optional): slug = subdirectory name
#LIBRARY_NAMES=mylib:My Home Library,other:Public Domain
# Rescan guard: max percent of a library's rows a scan may tombstone (100 disables)
LIBRARY_MISSING_THRESHOLD=30
```

## Usage

TBD

## Docker Deployment

See [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md) for detailed Docker deployment instructions.

### Quick Docker Start

```bash
# Build and run
docker compose up -d

# Initialize database
docker compose exec bopds /app/bopds init

# Scan all libraries
sudo docker compose exec bopds /app/bopds scan

# Or a single library by name
sudo docker compose exec bopds /app/bopds scan mylib
```

## Library Structure

`LIBRARY_PATH` is a PATH-style list of roots. Each root's first-level
subdirectories are libraries; the subdirectory name is the library name
(overridable via `LIBRARY_NAMES`). Inside a library, books come from either
index source:

- `name.inpx` — a zip containing `name.inp` index lines (the classic source)
- `name.inp` — a loose index file with the same line format

The companion archive (`name.zip` or `name.7z`) can sit anywhere in the
library tree. A checked-in sample fixture demonstrating both styles lives in
[`testdata/libraries/`](testdata/libraries/README.md).

Supported archive formats:

- `.zip` archives containing `.fb2` files
- `.7z` archives containing `.fb2` files

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [FictionBook 2.0](https://github.com/gribuser/fb2) specification
- [OPDS](https://opds.io/) protocol
- [SQLite](https://www.sqlite.org/) database
- [Vue.js](https://vuejs.org/) framework
- [Tailwind CSS](https://tailwindcss.com/) utility-first CSS framework

## CI/CD

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Support

For issues, questions, or contributions, please visit:

- GitHub Issues: <https://github.com/htol/bopds/issues>
- Documentation: See AGENTS.md for development guidelines
