package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type FractalParams struct {
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	CenterX float64 `json:"centerX"`
	CenterY float64 `json:"centerY"`
	Zoom    float64 `json:"zoom"`
	MaxIter int     `json:"maxIter"`
}

func main() {
	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API endpoints
	http.HandleFunc("/api/fractal", handleFractal)
	http.HandleFunc("/api/health", handleHealth)

	// Root handler
	http.HandleFunc("/", handleRoot)

	// Get port from environment variable, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	port = ":" + port
	
	log.Printf("Fractals server starting on port %s", port)
	log.Printf("Access the application at http://localhost%s", port)
	
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	// Allow root path and empty path (for proxy setups)
	if r.URL.Path != "/" && r.URL.Path != "" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Fractals v1</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #1a1a1a;
            color: #e0e0e0;
        }
        h1 {
            color: #4a9eff;
        }
        .container {
            background: #2a2a2a;
            padding: 20px;
            border-radius: 8px;
            margin-top: 20px;
        }
        canvas {
            border: 1px solid #444;
            background: #000;
            display: block;
            margin: 20px auto;
        }
        .controls {
            display: flex;
            gap: 10px;
            flex-wrap: wrap;
            margin: 20px 0;
        }
        input, button {
            padding: 8px;
            border: 1px solid #555;
            border-radius: 4px;
            background: #333;
            color: #e0e0e0;
        }
        button {
            cursor: pointer;
            background: #4a9eff;
            color: white;
            border: none;
        }
        button:hover {
            background: #3a8eef;
        }
    </style>
</head>
<body>
    <h1>Fractals v1</h1>
    <div class="container">
        <canvas id="fractalCanvas" width="800" height="600"></canvas>
        <div class="controls">
            <button onclick="generateFractal()">Generate Fractal</button>
            <button onclick="resetView()">Reset View</button>
        </div>
    </div>
    <script>
        const canvas = document.getElementById('fractalCanvas');
        const ctx = canvas.getContext('2d');
        
        // Get the base path from the current location
        // When accessed via /fractals/v1, pathname will be /fractals/v1
        // Use the full URL and extract the path to ensure we get the correct base
        const currentUrl = new URL(window.location.href);
        let basePath = currentUrl.pathname;
        // Remove trailing slash if present (but keep root /)
        if (basePath.endsWith('/') && basePath.length > 1) {
            basePath = basePath.slice(0, -1);
        }
        // Debug: log the base path
        console.log('Current URL:', window.location.href);
        console.log('Base path detected:', basePath);
        
        let params = {
            width: 800,
            height: 600,
            centerX: 0.0,
            centerY: 0.0,
            zoom: 1.0,
            maxIter: 100
        };

        function generateFractal() {
            params.width = canvas.width;
            params.height = canvas.height;
            
            // Show loading indicator
            ctx.fillStyle = '#333';
            ctx.fillRect(0, 0, canvas.width, canvas.height);
            ctx.fillStyle = '#fff';
            ctx.font = '20px Arial';
            ctx.textAlign = 'center';
            ctx.fillText('Generating fractal...', canvas.width / 2, canvas.height / 2);
            
            // Construct the API URL using the basePath we extracted
            // This ensures we maintain the full path including /fractals/v1
            const apiUrl = window.location.origin + basePath + '/api/fractal';
            console.log('Base path:', basePath);
            console.log('Constructed API URL:', apiUrl);
            console.log('Will fetch from:', apiUrl);
            
            fetch(apiUrl, {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(params)
            })
            .then(response => {
                if (!response.ok) {
                    throw new Error('HTTP error! status: ' + response.status);
                }
                return response.json();
            })
            .then(data => {
                if (data.error) {
                    console.error('Error:', data.error);
                    ctx.fillStyle = '#f00';
                    ctx.fillText('Error: ' + data.error, canvas.width / 2, canvas.height / 2);
                    return;
                }
                drawFractal(data);
            })
            .catch(error => {
                console.error('Error:', error);
                ctx.fillStyle = '#f00';
                ctx.fillText('Error: ' + error.message, canvas.width / 2, canvas.height / 2);
            });
        }

        function drawFractal(data) {
            const imageData = ctx.createImageData(canvas.width, canvas.height);
            for (let i = 0; i < data.pixels.length; i++) {
                const pixel = data.pixels[i];
                const idx = i * 4;
                imageData.data[idx] = pixel.r;
                imageData.data[idx + 1] = pixel.g;
                imageData.data[idx + 2] = pixel.b;
                imageData.data[idx + 3] = 255;
            }
            ctx.putImageData(imageData, 0, 0);
        }

        function resetView() {
            params.centerX = 0.0;
            params.centerY = 0.0;
            params.zoom = 1.0;
            generateFractal();
        }

        // Generate initial fractal
        generateFractal();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

func handleFractal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var params FractalParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Set defaults
	if params.Width == 0 {
		params.Width = 800
	}
	if params.Height == 0 {
		params.Height = 600
	}
	if params.MaxIter == 0 {
		params.MaxIter = 100
	}
	if params.Zoom == 0 {
		params.Zoom = 1.0
	}

	// Generate Mandelbrot set fractal
	pixels := generateMandelbrot(params)

	response := map[string]interface{}{
		"pixels": pixels,
		"params": params,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func generateMandelbrot(params FractalParams) []map[string]int {
	pixels := make([]map[string]int, params.Width*params.Height)
	
	scale := 4.0 / params.Zoom
	xOffset := params.CenterX
	yOffset := params.CenterY

	for y := 0; y < params.Height; y++ {
		for x := 0; x < params.Width; x++ {
			// Map pixel coordinates to complex plane
			cx := (float64(x)/float64(params.Width))*scale - scale/2 + xOffset
			cy := (float64(y)/float64(params.Height))*scale - scale/2 + yOffset

			// Calculate Mandelbrot iteration
			iter := mandelbrotIteration(cx, cy, params.MaxIter)

			// Color based on iteration count
			r, g, b := colorFromIteration(iter, params.MaxIter)

			idx := y*params.Width + x
			pixels[idx] = map[string]int{
				"r": r,
				"g": g,
				"b": b,
			}
		}
	}

	return pixels
}

func mandelbrotIteration(cx, cy float64, maxIter int) int {
	var zx, zy float64
	for i := 0; i < maxIter; i++ {
		if zx*zx+zy*zy > 4.0 {
			return i
		}
		zx, zy = zx*zx-zy*zy+cx, 2.0*zx*zy+cy
	}
	return maxIter
}

func colorFromIteration(iter, maxIter int) (int, int, int) {
	if iter == maxIter {
		return 0, 0, 0 // Black for points in the set
	}

	// Create a colorful gradient
	t := float64(iter) / float64(maxIter)
	
	r := int(9 * (1 - t) * t * t * t * 255)
	g := int(15 * (1 - t) * (1 - t) * t * t * 255)
	b := int(8.5 * (1 - t) * (1 - t) * (1 - t) * t * 255)

	if r > 255 {
		r = 255
	}
	if g > 255 {
		g = 255
	}
	if b > 255 {
		b = 255
	}

	return r, g, b
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "fractals/v1",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

