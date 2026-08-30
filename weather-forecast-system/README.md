# Weather Forecast System

## Project

Weather Forecast System

## Development Method

Agile

## Current Sprint

Sprint 1 — Initial Backend Development

## Sprint Goal

The goal of Sprint 1 is to establish the initial backend foundation and implement a small, working weather-system capability. This increment demonstrates that real development work has started and provides a simple server that can be extended in later sprints.

## Technologies

- Go
- Gin

## Implemented

- Gin web server running locally on a development port
- GET /api/health endpoint returning a JSON status response
- GET /api/forecast endpoint returning sample development weather data for Sydney, Australia

## Future Sprints

Frontend development and additional system functionality will be implemented incrementally in later sprints.

## Running the Project

From the backend directory, run:

```bash
cd backend
go mod tidy
go run .
```

The server will start on:

```text
http://localhost:8080
```

## API Endpoints

### GET /api/health

Returns a JSON response confirming the backend is running.

### GET /api/forecast

Returns sample weather forecast data for Sydney, NSW, Australia. This payload is clearly marked as sample development data and does not connect to an external weather API.
