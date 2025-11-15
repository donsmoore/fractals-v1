# Fractals v1

A Go-based web application for generating and displaying fractals.

## Features

- Mandelbrot set fractal generation
- Interactive web interface
- RESTful API for fractal generation
- Real-time rendering

## Requirements

- Go 1.21 or higher

## Installation

1. Navigate to the project directory:
```bash
cd /var/www/html/fractals/v1
```

2. Install dependencies (if any):
```bash
go mod tidy
```

## Running the Server

Start the server:
```bash
go run main.go
```

The server will start on port 8080. Access it at:
- Web interface: http://localhost:8080
- Health check: http://localhost:8080/api/health

## API Endpoints

### POST /api/fractal
Generate a fractal with custom parameters.

Request body:
```json
{
  "width": 800,
  "height": 600,
  "centerX": 0.0,
  "centerY": 0.0,
  "zoom": 1.0,
  "maxIter": 100
}
```

### GET /api/health
Check server health status.

## Building

Build the application:
```bash
go build -o fractals main.go
```

Run the binary:
```bash
./fractals
```

## Project Structure

```
fractals/v1/
├── main.go      # Main application file
├── go.mod       # Go module definition
└── README.md    # This file
```

