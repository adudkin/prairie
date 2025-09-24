# prairie
Simple AI generated turn-based game

## Running with Docker

1. Build and start the services:
   ```bash
   docker compose up --build
   ```
2. Open the application in your browser: [http://localhost:8080](http://localhost:8080)

The API service stores state in a SQLite database whose location can be configured with the `PRAIRIE_DB_PATH` environment variable. By default it uses `/tmp/prairie.db`, which is writable by the non-root container user. To persist data between runs, override `PRAIRIE_DB_PATH` and mount a volume at that path.
