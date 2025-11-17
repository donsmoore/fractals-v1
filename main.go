package main

import (
	"encoding/json"
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"math/rand"
	"image"
	"image/png"
	"image/color"
	"bytes"
	"context"
	"runtime"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type FractalParams struct {
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	CenterX float64 `json:"centerX"`
	CenterY float64 `json:"centerY"`
	Zoom    float64 `json:"zoom"`
	MaxIter int     `json:"maxIter"`
	FractalType string `json:"fractalType"` // "mandelbrot", "julia", "newton", "ifs"
	Exponent float64 `json:"exponent"` // Power for z^n + c (Mandelbrot/Julia)
	JuliaCReal float64 `json:"juliaCReal"` // Julia set constant (real part)
	JuliaCImag float64 `json:"juliaCImag"` // Julia set constant (imaginary part)
	NewtonDegree int `json:"newtonDegree"` // Newton fractal polynomial degree (2-6)
	IFSType string `json:"ifsType"` // IFS type: "fern", "sierpinski", "dragon", "tree", "spiral"
	IFSPoints int `json:"ifsPoints"` // Number of points to generate for IFS
	ColorPalette string `json:"colorPalette"` // Color palette name: "classic", "random"
	Quality      string `json:"quality"`      // "low" | "medium" | "high" | "auto"
}

// Configuration for per-request random palette
type RandPaletteConfig struct {
    baseHue float64
    hueSpan float64
    satMin  float64
    satMax  float64
    valMin  float64
    valMax  float64
}

var renderSem = make(chan struct{}, 1) // limit to 1 concurrent render

// Tunables with env overrides
var (
    defaultMaxPixels  = envInt("FRAC_MAX_PIXELS", 300000) // tighter default cap ~0.3M px
    defaultMaxIter    = envInt("FRAC_MAX_ITER", 90)
    renderConcurrency = envInt("FRAC_MAX_CONCURRENCY", 1)
)

func init() {
    // reset semaphore size from env
    if renderConcurrency < 1 { renderConcurrency = 1 }
    renderSem = make(chan struct{}, renderConcurrency)
}

func envInt(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 { return n }
    }
    return def
}

func main() {
    // Serve static files
    fs := http.FileServer(http.Dir("./static"))
    http.Handle("/static/", http.StripPrefix("/static/", fs))

    // API endpoints
    http.HandleFunc("/api/fractal", handleFractal)
    http.HandleFunc("/api/health", handleHealth)
    http.HandleFunc("/favicon.ico", handleFavicon)

    // Root handler
    http.HandleFunc("/", handleRoot)

    // Get port from environment variable, default to 8080
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    addr := ":" + port
    log.Printf("Fractals server starting on port %s", addr)
    log.Printf("Access the application at http://localhost%s", addr)

    srv := &http.Server{
        Addr:              addr,
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      20 * time.Second,
        IdleTimeout:       60 * time.Second,
        Handler:           http.DefaultServeMux,
    }
    if err := srv.ListenAndServe(); err != nil {
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
    <link rel="icon" type="image/png" href="/favicon.ico">
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            background: #1a1a1a;
            color: #e0e0e0;
            height: 100vh;
            overflow: hidden;
        }
        h1 {
            color: #4a9eff;
        }
        .topbar {
            position: sticky;
            top: 0;
            z-index: 10;
            background: #2a2a2a;
            padding: 6px 10px; /* compact to button height */
            border-bottom: 1px solid #333;
        }
        .container {
            height: calc(100vh - 58px);
            display: flex;
            align-items: center;
            justify-content: center;
        }
        canvas {
            border: 1px solid #222;
            background: #000;
            display: block;
            /* width/height set dynamically to preserve aspect ratio */
        }
        .controls {
            display: flex;
            justify-content: space-between; /* left and right groups */
            gap: 10px;
            flex-wrap: nowrap;
            margin: 0;
            align-items: center;
        }
        .controls-left, .controls-right { display: flex; align-items: center; gap: 10px; }
        .info-overlay {
            font-size: 12px;
            color: #fff;
            background: rgba(0,0,0,0.6);
            padding: 4px 8px;
            border: 1px solid rgba(255,255,255,0.2);
            border-radius: 4px;
            text-shadow: 0 1px 2px rgba(0,0,0,0.85);
            line-height: 1.4;
        }
        .control-group {
            display: flex;
            flex-direction: column;
            gap: 4px;
            align-items: flex-start;
            margin-left: 10px;
        }
        .control-group label {
            color: #e0e0e0;
            font-size: 12px;
            white-space: nowrap;
            margin-bottom: 2px;
        }
        .control-group .control-row {
            display: flex;
            gap: 5px;
            align-items: center;
        }
        input, button {
            padding: 8px;
            border: 1px solid #555;
            border-radius: 4px;
            background: #333;
            color: #e0e0e0;
        }
        input[type="number"] {
            width: 72px; /* was 100px */
        }
        select {
            min-width: 140px; /* was 200px */
            max-width: 180px;
        }
        select option {
            padding: 5px;
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
    <div class="topbar">
        <div class="controls">
            <div class="controls-left">
                <div id="infoOverlay" class="info-overlay">Center: (0.0000, 0.0000)<br>Zoom: 1.00x</div>
                <div class="control-group">
                    <label for="fractalTypeSelect">Fractal Type</label>
                    <div class="control-row">
                        <select id="fractalTypeSelect" onchange="updateFractalType()" style="padding: 8px; border: 1px solid #555; border-radius: 4px; background: #333; color: #e0e0e0; cursor: pointer; min-width: 140px;">
                            <option value="mandelbrot">Mandelbrot</option>
                            <option value="julia">Julia Set</option>
                            <option value="newton">Newton Fractal</option>
                            <option value="fern">Barnsley Fern</option>
                            <option value="sierpinski">Sierpinski Triangle</option>
                            <option value="dragon">Dragon Curve</option>
                            <option value="tree">Tree</option>
                            <option value="spiral">Spiral</option>
                        </select>
                    </div>
                </div>
                <div class="control-group" id="exponentGroup">
                    <label for="exponentInput">Exponent (z^n + c)</label>
                    <div class="control-row">
                        <input type="number" id="exponentInput" step="0.1" min="1" max="10" value="2.0" onkeypress="if(event.key==='Enter') updateExponent()">
                        <button onclick="updateExponent()">Update</button>
                    </div>
                </div>
                <div class="control-group" id="juliaGroup" style="display:none;">
                    <label for="juliaCRealInput">Julia C</label>
                    <div class="control-row">
                        <input type="number" id="juliaCRealInput" step="0.01" value="-0.7269" placeholder="real" onkeypress="if(event.key==='Enter') updateJulia()">
                        <input type="number" id="juliaCImagInput" step="0.01" value="0.1889" placeholder="imag" onkeypress="if(event.key==='Enter') updateJulia()">
                        <button onclick="updateJulia()">Update</button>
                    </div>
                </div>
                <div class="control-group" id="newtonGroup" style="display:none;">
                    <label for="newtonDegreeSelect">Polynomial Degree</label>
                    <div class="control-row">
                        <select id="newtonDegreeSelect" onchange="updateNewton()" style="padding: 6px; border: 1px solid #555; border-radius: 4px; background: #333; color: #e0e0e0; min-width: 80px;">
                            <option value="2">2</option>
                            <option value="3" selected>3</option>
                            <option value="4">4</option>
                            <option value="5">5</option>
                            <option value="6">6</option>
                        </select>
                    </div>
                </div>
                <div class="control-group">
                    <label for="paletteSelect">Color Palette</label>
                    <div class="control-row">
                        <select id="paletteSelect" onchange="updatePalette()" style="padding: 8px; border: 1px solid #555; border-radius: 4px; background: #333; color: #e0e0e0; cursor: pointer;">
                            <option value="classic">Classic</option>
                            <option value="random">Random</option>
                            <option value="white">White</option>
                        </select>
                    </div>
                </div>
                <div class="control-group">
                    <label for="qualitySelect">Quality</label>
                    <div class="control-row">
                        <select id="qualitySelect" onchange="updateQuality()" style="padding: 6px; border: 1px solid #555; border-radius: 4px; background: #333; color: #e0e0e0; min-width: 120px; max-width: 140px;">
                            <option value="low">Low</option>
                            <option value="medium" selected>Medium</option>
                            <option value="high">High</option>
                            <option value="auto">Auto</option>
                        </select>
                    </div>
                </div>
            </div>
            <div class="controls-right">
                <button onclick="generateFractal()">Generate</button>
                <button onclick="resetView()">Reset</button>
                <button onclick="printFractal()">Export</button>
            </div>
        </div>
    </div>
    <div class="container">
        <canvas id="fractalCanvas"></canvas>
    </div>
    <script>
        const canvas = document.getElementById('fractalCanvas');
        const ctx = canvas.getContext('2d');
        const btnGen = document.querySelector('button[onclick="generateFractal()"]');
        const btnReset = document.querySelector('button[onclick="resetView()"]');
        const btnPrint = document.querySelector('button[onclick="printFractal()"]');
        // Determine client quality heuristic (auto)
        function detectQuality(){
            const dm = navigator.deviceMemory || 0; // in GiB (may be undefined)
            const hc = navigator.hardwareConcurrency || 1;
            const mp = Math.max(1, (window.innerWidth*window.innerHeight));
            // Heuristic: small memory or few cores or huge viewport => lower quality
            if (dm && dm < 2) return 'low';
            if (hc <= 2) return 'low';
            if (mp > 2_000_000) return 'medium';
            return 'medium';
        }
        let quality = detectQuality();
        // Rectangle zoom state
        let isSelecting = false;
        let selectStart = null; // { nx, ny }
        let lastRenderedImageData = null;
        let lastRandPalette = null; // cache of server-provided random palette params
        let lastUsedIterations = null; // server-reported effective iterations
        
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
            maxIter: 100,
            fractalType: 'mandelbrot',
            exponent: 2.0,  // Default exponent for standard Mandelbrot (z^2 + c)
            juliaCReal: -0.7269,
            juliaCImag: 0.1889,
            newtonDegree: 3,
            ifsType: 'fern',
            ifsPoints: 500,  // Always use maximum points for IFS
            colorPalette: 'classic'  // Default color palette
        };
        
        // Initialize the input field with default value
        document.addEventListener('DOMContentLoaded', function() {
            document.getElementById('fractalTypeSelect').value = params.fractalType;
            document.getElementById('exponentInput').value = params.exponent;
            document.getElementById('juliaCRealInput').value = params.juliaCReal;
            document.getElementById('juliaCImagInput').value = params.juliaCImag;
            document.getElementById('newtonDegreeSelect').value = params.newtonDegree;
            document.getElementById('paletteSelect').value = params.colorPalette;
            document.getElementById('qualitySelect').value = quality; // Initialize quality
            updateFractalType(); // Show/hide appropriate controls
        });

        function sizeCanvasToViewport() {
            // Fit a target aspect ratio (4:3) within available viewport
            const topbar = document.querySelector('.topbar');
            const availW = Math.floor(document.documentElement.clientWidth);
            const availH = Math.floor(document.documentElement.clientHeight - (topbar ? topbar.offsetHeight : 0));
            const targetAspect = 4 / 3; // preserve original 800x600

            let drawW = availW;
            let drawH = Math.floor(availW / targetAspect);
            if (drawH > availH) {
                drawH = availH;
                drawW = Math.floor(availH * targetAspect);
            }

            // Apply CSS size for layout and backing store for crispness
            canvas.style.width = drawW + 'px';
            canvas.style.height = drawH + 'px';
            canvas.width = drawW;
            canvas.height = drawH;
        }

        // Overlay updates
        function updateInfoOverlay() {
            const el = document.getElementById('infoOverlay');
            if (!el) return;
            const fmt = (n) => (Math.abs(n) < 1000 ? n.toFixed(4) : n.toExponential(2));
            el.innerHTML = 'Center: (' + fmt(params.centerX) + ', ' + fmt(params.centerY) + ')<br>Zoom: ' + params.zoom.toFixed(2) + 'x';
        }

        function setBusy(b){
            [btnGen, btnReset, btnPrint].forEach(el=>{ if(el) el.disabled = !!b; });
            canvas.style.cursor = b ? 'progress' : 'default';
        }

        function generateFractal() {
            setBusy(true);
            sizeCanvasToViewport();
            params.width = canvas.width;
            params.height = canvas.height;
            
            // Show loading indicator
            ctx.fillStyle = '#333';
            ctx.fillRect(0, 0, canvas.width, canvas.height);
            ctx.fillStyle = '#fff';
            ctx.font = '20px Arial';
            ctx.textAlign = 'center';
            ctx.fillText('Generating fractal...', canvas.width / 2, canvas.height / 2);
            updateInfoOverlay();
            
            // Construct the API URL
            // If basePath is "/" or empty, use direct path, otherwise use basePath
            let apiUrl;
            if (basePath === '/' || basePath === '') {
                apiUrl = window.location.origin + '/api/fractal';
            } else {
                apiUrl = window.location.origin + basePath + '/api/fractal';
            }
            console.log('Base path:', basePath);
            console.log('Constructed API URL:', apiUrl);
            console.log('Will fetch from:', apiUrl);
            
            var qSel = document.getElementById('qualitySelect');
            var qVal = qSel ? qSel.value : 'low';
            var req = {
                width: params.width,
                height: params.height,
                centerX: params.centerX,
                centerY: params.centerY,
                zoom: params.zoom,
                maxIter: params.maxIter,
                fractalType: params.fractalType,
                exponent: params.exponent,
                juliaCReal: params.juliaCReal,
                juliaCImag: params.juliaCImag,
                newtonDegree: params.newtonDegree,
                ifsType: (params.fractalType === 'fern' || params.fractalType === 'sierpinski' || params.fractalType === 'dragon' || params.fractalType === 'tree' || params.fractalType === 'spiral') ? params.fractalType : 'fern',
                ifsPoints: params.ifsPoints,
                colorPalette: params.colorPalette,
                quality: qVal
            };
            fetch(apiUrl, {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(req)
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
                updateInfoOverlay();
                // cache used iterations for consistent export
                if (data && typeof data.usedIterations === 'number') {
                    lastUsedIterations = data.usedIterations;
                }
            })
            .catch(error => {
                console.error('Error:', error);
                ctx.fillStyle = '#f00';
                ctx.fillText('Error: ' + error.message, canvas.width / 2, canvas.height / 2);
            })
            .finally(()=>{ setBusy(false); });
        }

        function drawFractal(data) {
            // Draw PNG returned by server, scaling to canvas
            const srcW = data.renderWidth || canvas.width;
            const srcH = data.renderHeight || canvas.height;
            const img = new Image();
            img.onload = function(){
                ctx.clearRect(0,0,canvas.width,canvas.height);
                ctx.drawImage(img, 0, 0, srcW, srcH, 0, 0, canvas.width, canvas.height);
                // cache for overlay drawing
                try { lastRenderedImageData = ctx.getImageData(0, 0, canvas.width, canvas.height); } catch (e) { lastRenderedImageData = null; }
                // cache palette info from server (to reuse for printing)
                if (data && data.randPalette) {
                    lastRandPalette = data.randPalette;
                }
            };
            img.src = 'data:image/png;base64,' + data.pngData;
        }

        function resetView() {
            // Only reset view parameters (center and zoom), keep fractal type and settings
            const currentFractalType = params.fractalType;
            
            // Reset view based on fractal type
            if (currentFractalType === 'ifs') {
                // Set center based on IFS type
                if (params.ifsType === 'dragon') {
                    // Dragon curve bounds: {0, 1.5, -0.5, 0.5}, center at (0.75, 0.0)
                    // Small offset to center the visual curve
                    params.centerX = 0.05;  // Small shift right
                    params.centerY = 0.15;  // Small shift up
                    params.zoom = 0.7;      // Zoom out to show entire curve
                } else {
                    params.centerX = 0.0;
                    params.centerY = 0.0;
                    params.zoom = 1.0;
                }
            } else {
                params.centerX = 0.0;
                params.centerY = 0.0;
                params.zoom = 1.0;
            }
            
            // Keep all other settings (fractal type, exponent, julia constants, etc.)
            // Just regenerate with reset view
            generateFractal();
        }
        
        function updateFractalType() {
            const fractalTypeSelect = document.getElementById('fractalTypeSelect');
            params.fractalType = fractalTypeSelect.value;
            
            // Show/hide controls based on fractal type
            document.getElementById('exponentGroup').style.display = 
                (params.fractalType === 'mandelbrot' || params.fractalType === 'julia') ? 'flex' : 'none';
            document.getElementById('juliaGroup').style.display = 
                (params.fractalType === 'julia') ? 'flex' : 'none';
            document.getElementById('newtonGroup').style.display = 
                (params.fractalType === 'newton') ? 'flex' : 'none';
            
            // Reset view for certain fractals
            if (params.fractalType === 'newton') {
                params.centerX = 0.0;
                params.centerY = 0.0;
                params.zoom = 1.0;
            } else if (params.fractalType === 'dragon') {
                // Dragon curve bounds: {0, 1.5, -0.5, 0.5}, center at (0.75, 0.0)
                // Small offset to center the visual curve
                params.centerX = 0.05;  // Small shift right
                params.centerY = 0.15;  // Small shift up
                params.zoom = 0.7;      // Zoom out to show entire curve
            } else if (params.fractalType === 'fern' || params.fractalType === 'sierpinski' || params.fractalType === 'tree' || params.fractalType === 'spiral') {
                // Other IFS types
                params.centerX = 0.0;
                params.centerY = 0.0;
                params.zoom = 1.0;
            } else {
                params.centerX = 0.0;
                params.centerY = 0.0;
                params.zoom = 1.0;
            }
            
            generateFractal();
        }
        
        function updateExponent() {
            const exponentInput = document.getElementById('exponentInput');
            const newExponent = parseFloat(exponentInput.value);
            
            if (isNaN(newExponent) || newExponent < 1 || newExponent > 10) {
                alert('Exponent must be a number between 1 and 10');
                exponentInput.value = params.exponent;
                return;
            }
            
            params.exponent = newExponent;
            generateFractal();
        }
        
        function updateJulia() {
            const juliaCRealInput = document.getElementById('juliaCRealInput');
            const juliaCImagInput = document.getElementById('juliaCImagInput');
            const newCReal = parseFloat(juliaCRealInput.value);
            const newCImag = parseFloat(juliaCImagInput.value);
            
            if (isNaN(newCReal) || isNaN(newCImag)) {
                alert('Julia constant must be valid numbers');
                juliaCRealInput.value = params.juliaCReal;
                juliaCImagInput.value = params.juliaCImag;
                return;
            }
            
            params.juliaCReal = newCReal;
            params.juliaCImag = newCImag;
            generateFractal();
        }
        
        function updateNewton() {
            const newtonDegreeSelect = document.getElementById('newtonDegreeSelect');
            params.newtonDegree = parseInt(newtonDegreeSelect.value);
            generateFractal();
        }
        
        
        function updatePalette() {
            const paletteSelect = document.getElementById('paletteSelect');
            params.colorPalette = paletteSelect.value;
            generateFractal();
        }
 
         function printFractal() {
            // Generate a high-res image client-side with current view parameters
            // Match current canvas aspect ratio; cap to 4K on long edge
            const cx = params.centerX;
            const cy = params.centerY;
            const zoom = params.zoom;
            const fractalType = params.fractalType || 'mandelbrot';
            const exponent = params.exponent || 2.0;
            const juliaCReal = params.juliaCReal || -0.7269;
            const juliaCImag = params.juliaCImag || 0.1889;
            const newtonDegree = params.newtonDegree || 3;
            const palette = params.colorPalette || 'classic';
            const effIter = (typeof lastUsedIterations === 'number' && lastUsedIterations>0) ? lastUsedIterations : params.maxIter;

            const aspect = canvas.width / Math.max(1, canvas.height);
            let exportW, exportH;
            const maxW = 3840, maxH = 2160;
            if (aspect >= 1) {
                // landscape-ish
                exportW = maxW;
                exportH = Math.round(exportW / aspect);
                if (exportH > maxH) { exportH = maxH; exportW = Math.round(exportH * aspect); }
            } else {
                // portrait-ish (unlikely, but handle)
                exportH = maxH;
                exportW = Math.round(exportH * aspect);
                if (exportW > maxW) { exportW = maxW; exportH = Math.round(exportW / aspect); }
            }

            const w = window.open('about:blank', '_blank');
            if (!w) return; // popup blocked
            w.document.write('<!DOCTYPE html><html><head><title>Fractal 4K Export</title><style>html,body{margin:0;height:100%;background:#000;color:#e0e0e0;font-family:Arial,sans-serif} .msg{position:fixed;top:50%;left:50%;transform:translate(-50%,-50%);font-size:16px;background:rgba(0,0,0,0.7);padding:12px 16px;border:1px solid #444;border-radius:4px;text-align:center;min-width:280px} .bar{margin-top:10px;width:100%;height:8px;background:#222;border:1px solid #555;border-radius:4px;overflow:hidden} .fill{height:100%;width:0;background:#4a9eff;transition:width .1s linear} .pct{display:block;margin-top:6px;font-size:12px;color:#ccc} .toolbar{position:fixed;top:10px;right:10px;z-index:5} .btn{padding:8px 12px;border:1px solid #555;border-radius:4px;background:#333;color:#e0e0e0;text-decoration:none} img{display:block;width:100vw;height:100vh;object-fit:contain;background:#000;}</style></head><body>');
            w.document.write('<div id="msg" class="msg">Generating high resolution image...<div class="bar"><div id="fill" class="fill"></div></div><span id="pct" class="pct">0%</span></div>');
            w.document.write('</body></html>');
            w.document.close();

            // Build in the opened window context using a Web Worker for generation
            (function(doc){
                function clamp255(v){return v<0?0:(v>255?255:v);} 
                function lerp(a,b,t){return a+(b-a)*t;}
                function hsvToRgb(h,s,v){
                    var c=v*s;
                    var x=c*(1-Math.abs(((h/60)%2)-1));
                    var m=v-c;var r1=0,g1=0,b1=0;
                    if(h<60){r1=c;g1=x;b1=0;} else if(h<120){r1=x;g1=c;b1=0;} else if(h<180){r1=0;g1=c;b1=x;} else if(h<240){r1=0;g1=x;b1=c;} else if(h<300){r1=x;g1=0;b1=c;} else {r1=c;g1=0;b1=x;}
                    return [clamp255(Math.round((r1+m)*255)),clamp255(Math.round((g1+m)*255)),clamp255(Math.round((b1+m)*255))];
                }
                // Generate on offscreen canvas with chunked updates in Web Worker
                var off=doc.createElement('canvas');
                off.width=exportW; off.height=exportH;
                var octx=off.getContext('2d');
                var img=octx.createImageData(exportW,exportH);
                var scale=4.0/zoom;
                var msgEl=doc.getElementById('msg');
                var fillEl=doc.getElementById('fill');
                var pctEl=doc.getElementById('pct');
                // Build worker code
                var workerCode = ''+
                'self.onmessage=function(e){\n'+
                ' var d=e.data;\n'+
                ' var exportW=d.exportW, exportH=d.exportH, cx=d.cx, cy=d.cy, zoom=d.zoom, maxIter=d.maxIter, exponent=d.exponent, palette=d.palette, randCfg=d.randCfg;\n'+
                ' function clamp255(v){return v<0?0:(v>255?255:v);}\n'+
                ' function lerp(a,b,t){ return a+(b-a)*t }\n'+
                ' function hsvToRgb(h,s,v){'+
                '  var c=v*s; var x=c*(1-Math.abs((h/60)%2-1)); var m=v-c; var r1=0,g1=0,b1=0;'+
                '  if(h<60){r1=c; g1=x; b1=0;} else if(h<120){r1=x; g1=c; b1=0;} else if(h<180){r1=0; g1=c; b1=x;} else if(h<240){r1=0; g1=x; b1=c;} else if(h<300){r1=x; g1=0; b1=c;} else { r1=c; g1=0; b1=x; }'+
                '  var r=Math.round((r1+m)*255), g=Math.round((g1+m)*255), b=Math.round((b1+m)*255);'+
                '  return [clamp255(r),clamp255(g),clamp255(b)]; }\n'+
                ' var fractalType=d.fractalType||"mandelbrot"; var juliaCReal=d.juliaCReal||-0.7269; var juliaCImag=d.juliaCImag||0.1889; var newtonDegree=d.newtonDegree||3;\n'+
                ' function mandelIter(cx,cy,maxIter,exp){var zx=0,zy=0;for(var i=0;i<maxIter;i++){if(zx*zx+zy*zy>4.0) return i; var r=Math.hypot(zx,zy); if(r===0){zx=cx;zy=cy;continue;} var theta=Math.atan2(zy,zx); var newR=Math.pow(r,exp); var newTheta=theta*exp; zx=newR*Math.cos(newTheta)+cx; zy=newR*Math.sin(newTheta)+cy;} return maxIter;}\n'+
                ' function juliaIter(zx,zy,cr,ci,maxIter,exp){for(var i=0;i<maxIter;i++){if(zx*zx+zy*zy>4.0) return i; var r=Math.hypot(zx,zy); if(r===0) continue; var theta=Math.atan2(zy,zx); var newR=Math.pow(r,exp); var newTheta=theta*exp; zx=newR*Math.cos(newTheta)+cr; zy=newR*Math.sin(newTheta)+ci;} return maxIter;}\n'+
                ' function newtonIter(zx,zy,degree,maxIter){var roots=[]; for(var k=0;k<degree;k++){var angle=(2*Math.PI*k)/degree; roots.push({r:Math.cos(angle),i:Math.sin(angle)});} for(var i=0;i<maxIter;i++){var minDist=1e10,closestRoot=-1; for(var k=0;k<degree;k++){var dr=zx-roots[k].r,di=zy-roots[k].i,dist=dr*dr+di*di; if(dist<minDist){minDist=dist;closestRoot=k;}} if(minDist<1e-6) return closestRoot*maxIter/degree; var r=Math.hypot(zx,zy); if(r<1e-10) return maxIter; var theta=Math.atan2(zy,zx); var rn=Math.pow(r,degree),nTheta=theta*degree,znReal=rn*Math.cos(nTheta),znImag=rn*Math.sin(nTheta); var rn1=Math.pow(r,degree-1),n1Theta=theta*(degree-1),zn1Real=rn1*Math.cos(n1Theta),zn1Imag=rn1*Math.sin(n1Theta); var fReal=znReal-1,fImag=znImag; var fpReal=degree*zn1Real,fpImag=degree*zn1Imag; var fpMag2=fpReal*fpReal+fpImag*fpImag; if(fpMag2<1e-10) return maxIter; var fOverFpReal=(fReal*fpReal+fImag*fpImag)/fpMag2,fOverFpImag=(fImag*fpReal-fReal*fpImag)/fpMag2; zx=zx-fOverFpReal; zy=zy-fOverFpImag; if(zx*zx+zy*zy>100) return maxIter;} var minDist=1e10,closestRoot=0; for(var k=0;k<degree;k++){var dr=zx-roots[k].r,di=zy-roots[k].i,dist=dr*dr+di*di; if(dist<minDist){minDist=dist;closestRoot=k;}} return closestRoot*maxIter/degree;}\n'+
                ' function seededRandom(seed){var x=Math.sin(seed)*10000; return x-Math.floor(x);}\n'+
                ' function ifsIter(x,y,maxIter){var px=x,py=y; var seed=x*10000+y*10000; var rng=function(){seed=(seed*9301+49297)%233280; return seededRandom(seed);}; for(var i=0;i<maxIter;i++){var r=rng(); var nx,ny; if(r<0.01){nx=0;ny=0.16*py;} else if(r<0.86){nx=0.85*px+0.04*py;ny=-0.04*px+0.85*py+1.6;} else if(r<0.93){nx=0.20*px-0.26*py;ny=0.23*px+0.22*py+1.6;} else{nx=-0.15*px+0.28*py;ny=0.26*px+0.24*py+0.44;} px=nx;py=ny; if(px*px+py*py>100) return i;} return maxIter;}\n'+
                ' var img=new Uint8ClampedArray(exportW*exportH*4); var scale=4.0/zoom;\n'+
                ' var y=0; var chunk=32;\n'+
                ' function colorFromIter(it){ if(it>=maxIter) return [0,0,0]; var t=it/maxIter; if(palette==="random" && randCfg){ var h=( (randCfg.baseHue + randCfg.hueSpan*t) % 360 ); var s=lerp(randCfg.satMin, randCfg.satMax, t); var v=lerp(randCfg.valMin, randCfg.valMax, t); return hsvToRgb(h,s,v);} if(palette==="white"){ var b=Math.floor(255*t); return [b,b,b];} return hsvToRgb(360*t,1,1);}\n'+
                ' function step(){ var end=Math.min(exportH, y+chunk); for(var yy=y; yy<end; yy++){ var ny=yy/exportH; for(var x=0;x<exportW;x++){ var nx=x/exportW; var it=maxIter; if(fractalType==="mandelbrot"){var cxp=nx*scale - scale/2 + cx; var cyp=ny*scale - scale/2 + cy; it=mandelIter(cxp,cyp,maxIter,exponent);} else if(fractalType==="julia"){var cxp=nx*scale - scale/2 + cx; var cyp=ny*scale - scale/2 + cy; it=juliaIter(cxp,cyp,juliaCReal,juliaCImag,maxIter,exponent);} else if(fractalType==="newton"){var cxp=nx*scale - scale/2 + cx; var cyp=ny*scale - scale/2 + cy; it=newtonIter(cxp,cyp,newtonDegree,maxIter);} else if(fractalType==="fern"||fractalType==="sierpinski"||fractalType==="dragon"||fractalType==="tree"||fractalType==="spiral"){var cxp=nx*scale - scale/2 + cx; var cyp=ny*scale - scale/2 + cy; it=ifsIter(cxp,cyp,maxIter);} var rgb=colorFromIter(it); var idx=(yy*exportW+x)*4; img[idx]=rgb[0]; img[idx+1]=rgb[1]; img[idx+2]=rgb[2]; img[idx+3]=255; } } y= end; var p=Math.floor((y/exportH)*100); postMessage({type:"progress", p:p}); if(y<exportH){ setTimeout(step,0); } else { postMessage({type:"done", pixels: img, width: exportW, height: exportH }); } }\n'+
                ' step();\n'+
                '};';

                var blob = new Blob([workerCode], {type:'application/javascript'});
                var worker = new Worker(URL.createObjectURL(blob));
                worker.onmessage = function(ev){
                    if(ev.data && ev.data.type === 'progress'){
                        var p = ev.data.p|0; if(fillEl){fillEl.style.width=p+'%';} if(pctEl){pctEl.textContent=p+'%';}
                    } else if (ev.data && ev.data.type === 'done'){
                        // draw received pixels to offscreen then show
                        var w0 = ev.data.width, h0 = ev.data.height;
                        var id = octx.createImageData(w0,h0);
                        id.data.set(ev.data.pixels);
                        octx.putImageData(id,0,0);
                        var url = off.toDataURL('image/png');
                        doc.open();
                        doc.write('<!DOCTYPE html><html><head><title>Fractal 4K Export</title><style>html,body{margin:0;height:100%;background:#000;} .toolbar{position:fixed;top:10px;right:10px;z-index:5} .btn{padding:8px 12px;border:1px solid #555;border-radius:4px;background:#333;color:#e0e0e0;text-decoration:none} img{display:block;width:100vw;height:100vh;object-fit:contain;background:#000;}</style></head><body>');
                        doc.write('<div class="toolbar"><a id="dl" class="btn" download="fractal-4k.png" href="'+url+'">Download PNG (4K)</a></div>');
                        doc.write('<img src="'+url+'" alt="Fractal 4K"/>');
                        doc.write('<scr'+'ipt>window.addEventListener("load",function(){var a=document.getElementById("dl"); if(a){try{a.click();}catch(e){}}});</scr'+'ipt>');
                        doc.write('</body></html>');
                        doc.close();
                        worker.terminate();
                    }
                };
                var initRand = null;
                if ('random' === palette && typeof lastRandPalette === 'object' && lastRandPalette){ initRand = lastRandPalette; }
                worker.postMessage({exportW: exportW, exportH: exportH, cx: cx, cy: cy, zoom: zoom, maxIter: effIter, fractalType: fractalType, exponent: exponent, juliaCReal: juliaCReal, juliaCImag: juliaCImag, newtonDegree: newtonDegree, palette: palette, randCfg: initRand});
            })(w.document);
         }

        function updateQuality(){
            // apply new quality immediately
            generateFractal();
        }

        // Map normalized canvas coordinates to complex plane at current view
        function normToComplex(nx, ny) {
            // IFS fractals use different coordinate systems
            if (params.fractalType === 'fern' || params.fractalType === 'sierpinski' || params.fractalType === 'dragon' || params.fractalType === 'tree' || params.fractalType === 'spiral') {
                // Get bounds for current IFS type
                let bounds = { minX: -2.5, maxX: 2.5, minY: 0, maxY: 10 };
                switch (params.fractalType) {
                    case 'fern':
                        bounds = { minX: -2.5, maxX: 2.5, minY: 0, maxY: 10 };
                        break;
                    case 'sierpinski':
                        bounds = { minX: 0, maxX: 1, minY: 0, maxY: 1 };
                        break;
                    case 'dragon':
                        bounds = { minX: 0, maxX: 1.5, minY: -0.5, maxY: 0.5 };
                        break;
                    case 'tree':
                        bounds = { minX: -1, maxX: 1, minY: 0, maxY: 2 };
                        break;
                    case 'spiral':
                        bounds = { minX: -8, maxX: 3, minY: -1, maxY: 3 };
                        break;
                }
                
                // Calculate view bounds (matching server-side logic)
                const baseWidth = bounds.maxX - bounds.minX;
                const baseHeight = bounds.maxY - bounds.minY;
                const viewWidth = baseWidth / params.zoom;
                const viewHeight = baseHeight / params.zoom;
                const centerX = (bounds.minX + bounds.maxX) / 2;
                const centerY = (bounds.minY + bounds.maxY) / 2;
                const viewMinX = centerX + params.centerX - viewWidth / 2;
                const viewMaxX = centerX + params.centerX + viewWidth / 2;
                const viewMinY = centerY + params.centerY - viewHeight / 2;
                const viewMaxY = centerY + params.centerY + viewHeight / 2;
                
                // Map normalized coordinates to IFS coordinate space
                const cx = viewMinX + nx * (viewMaxX - viewMinX);
                const cy = viewMinY + ny * (viewMaxY - viewMinY);
                return { cx, cy };
            } else {
                // Standard complex plane mapping for Mandelbrot/Julia/Newton
                const scale = 4.0 / params.zoom;
                const cx = nx * scale - scale / 2 + params.centerX;
                const cy = ny * scale - scale / 2 + params.centerY;
                return { cx, cy };
            }
        }

        // Draw dashed selection rectangle maintaining canvas aspect ratio
        function drawSelectionOverlay(nx1, ny1, nx2, ny2) {
            if (lastRenderedImageData) ctx.putImageData(lastRenderedImageData, 0, 0);
            const targetAspect = canvas.width / canvas.height;
            const cxn = (nx1 + nx2) / 2;
            const cyn = (ny1 + ny2) / 2;
            let halfW = Math.abs(nx2 - nx1) / 2;
            let halfH = Math.abs(ny2 - ny1) / 2;
            if (halfW === 0 && halfH === 0) return;
            if (halfW / Math.max(halfH, 1e-9) > targetAspect) {
                halfH = halfW / targetAspect;
            } else {
                halfW = halfH * targetAspect;
            }
            // Clamp to [0,1]
            halfW = Math.min(halfW, cxn, 1 - cxn);
            halfH = Math.min(halfH, cyn, 1 - cyn);
            const left = (cxn - halfW) * canvas.width;
            const top = (cyn - halfH) * canvas.height;
            const w = (2 * halfW) * canvas.width;
            const h = (2 * halfH) * canvas.height;
            ctx.save();
            ctx.strokeStyle = '#ffffff';
            ctx.setLineDash([8, 6]);
            ctx.lineWidth = 2;
            ctx.strokeRect(Math.round(left), Math.round(top), Math.round(w), Math.round(h));
            ctx.restore();
        }

        // Apply zoom from two normalized points
        function applySelectionZoom(nx1, ny1, nx2, ny2) {
            const targetAspect = canvas.width / canvas.height;
            const cxn = (nx1 + nx2) / 2;
            const cyn = (ny1 + ny2) / 2;
            let halfW = Math.abs(nx2 - nx1) / 2;
            let halfH = Math.abs(ny2 - ny1) / 2;
            if (halfW === 0 && halfH === 0) return;
            if (halfW / Math.max(halfH, 1e-9) > targetAspect) {
                halfH = halfW / targetAspect;
            } else {
                halfW = halfH * targetAspect;
            }
            // Clamp
            halfW = Math.min(halfW, cxn, 1 - cxn);
            halfH = Math.min(halfH, cyn, 1 - cyn);
            const widthNorm = Math.max(2 * halfW, 1e-6);
            
            // For IFS fractals, need to calculate new center and zoom differently
            if (params.fractalType === 'fern' || params.fractalType === 'sierpinski' || params.fractalType === 'dragon' || params.fractalType === 'tree' || params.fractalType === 'spiral') {
                // Get bounds for current IFS type
                let bounds = { minX: -2.5, maxX: 2.5, minY: 0, maxY: 10 };
                switch (params.fractalType) {
                    case 'fern':
                        bounds = { minX: -2.5, maxX: 2.5, minY: 0, maxY: 10 };
                        break;
                    case 'sierpinski':
                        bounds = { minX: 0, maxX: 1, minY: 0, maxY: 1 };
                        break;
                    case 'dragon':
                        bounds = { minX: 0, maxX: 1.5, minY: -0.5, maxY: 0.5 };
                        break;
                    case 'tree':
                        bounds = { minX: -1, maxX: 1, minY: 0, maxY: 2 };
                        break;
                    case 'spiral':
                        bounds = { minX: -8, maxX: 3, minY: -1, maxY: 3 };
                        break;
                }
                
                // Calculate current view bounds
                const baseWidth = bounds.maxX - bounds.minX;
                const baseHeight = bounds.maxY - bounds.minY;
                const viewWidth = baseWidth / params.zoom;
                const viewHeight = baseHeight / params.zoom;
                const centerX = (bounds.minX + bounds.maxX) / 2;
                const centerY = (bounds.minY + bounds.maxY) / 2;
                const viewMinX = centerX + params.centerX - viewWidth / 2;
                const viewMaxX = centerX + params.centerX + viewWidth / 2;
                const viewMinY = centerY + params.centerY - viewHeight / 2;
                const viewMaxY = centerY + params.centerY + viewHeight / 2;
                
                // Calculate selected region center in IFS coordinates
                const selCenterX = viewMinX + cxn * (viewMaxX - viewMinX);
                const selCenterY = viewMinY + cyn * (viewMaxY - viewMinY);
                
                // Calculate new center relative to IFS center
                params.centerX = selCenterX - centerX;
                params.centerY = selCenterY - centerY;
                params.zoom = Math.max(0.0001, params.zoom / widthNorm);
            } else {
                // Standard complex plane mapping
                const center = normToComplex(cxn, cyn);
                params.centerX = center.cx;
                params.centerY = center.cy;
                params.zoom = Math.max(0.0001, params.zoom / widthNorm);
            }
            generateFractal();
        }

        // Selection interactions
        canvas.addEventListener('click', (e) => {
            const rect = canvas.getBoundingClientRect();
            const nx = (e.clientX - rect.left) / rect.width;
            const ny = (e.clientY - rect.top) / rect.height;
            if (!isSelecting) {
                isSelecting = true;
                selectStart = { nx, ny };
            } else {
                isSelecting = false;
                if (lastRenderedImageData) ctx.putImageData(lastRenderedImageData, 0, 0);
                applySelectionZoom(selectStart.nx, selectStart.ny, nx, ny);
                selectStart = null;
            }
        });

        canvas.addEventListener('mousemove', (e) => {
            if (!isSelecting || !selectStart) return;
            const rect = canvas.getBoundingClientRect();
            const nx = (e.clientX - rect.left) / rect.width;
            const ny = (e.clientY - rect.top) / rect.height;
            drawSelectionOverlay(selectStart.nx, selectStart.ny, nx, ny);
        });

        canvas.addEventListener('mouseleave', () => {
            if (isSelecting && lastRenderedImageData) {
                ctx.putImageData(lastRenderedImageData, 0, 0);
            }
        });


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
	if params.FractalType == "" {
		params.FractalType = "mandelbrot"
	}
	if params.Exponent == 0 {
		params.Exponent = 2.0 // Default to standard Mandelbrot (z^2 + c)
	}
	// Validate exponent range (1-10 for reasonable results)
	if params.Exponent < 1.0 {
		params.Exponent = 1.0
	}
	if params.Exponent > 10.0 {
		params.Exponent = 10.0
	}
	// Set default Julia constant if not provided
	if params.JuliaCReal == 0 && params.JuliaCImag == 0 {
		params.JuliaCReal = -0.7269
		params.JuliaCImag = 0.1889
	}
	// Set default Newton degree
	if params.NewtonDegree == 0 {
		params.NewtonDegree = 3
	}
	if params.NewtonDegree < 2 {
		params.NewtonDegree = 2
	}
	if params.NewtonDegree > 6 {
		params.NewtonDegree = 6
	}
	// Set default IFS parameters
	// Always use maximum points for IFS fractals
	params.IFSPoints = 500
	// Set IFSType from FractalType for IFS fractals (for backward compatibility)
	if params.FractalType == "fern" || params.FractalType == "sierpinski" || params.FractalType == "dragon" || params.FractalType == "tree" || params.FractalType == "spiral" {
		params.IFSType = params.FractalType
	} else if params.IFSType == "" {
		params.IFSType = "fern"
	}
	// Set default color palette
	if params.ColorPalette == "" {
		params.ColorPalette = "classic"
	}

	// Optional random palette configuration for this request
	var randCfg *RandPaletteConfig
	var randObj map[string]float64
	if params.ColorPalette == "random" {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		randCfg = &RandPaletteConfig{
			baseHue: r.Float64() * 360.0,
			hueSpan: 180.0 + r.Float64()*180.0, // 180..360
			satMin:  0.6,
			satMax:  1.0,
			valMin:  0.8,
			valMax:  1.0,
		}
		randObj = map[string]float64{
			"baseHue": randCfg.baseHue,
			"hueSpan": randCfg.hueSpan,
			"satMin":  randCfg.satMin,
			"satMax":  randCfg.satMax,
			"valMin":  randCfg.valMin,
			"valMax":  randCfg.valMax,
		}
	}

	// Enforce server-side pixel and iteration caps for low-resource environments, with quality mapping
	// Map quality to caps
	maxPx := defaultMaxPixels
	maxIt := defaultMaxIter
	switch strings.ToLower(params.Quality) {
	case "low":
		// very constrained: ~120k px, 60 iters
		if defaultMaxPixels > 120000 { maxPx = 120000 } else { maxPx = defaultMaxPixels }
		if defaultMaxIter > 60 { maxIt = 60 }
	case "medium":
		// use defaults (approx 0.3M px, 90 iters)
	case "high":
		maxPx = int(float64(defaultMaxPixels) * 2)
		if maxPx < 600000 { maxPx = 600000 }
		if maxIt < 120 { maxIt = 120 }
	case "auto", "":
		// leave defaults
	}
	reqW := params.Width
	reqH := params.Height
	if reqW <= 0 { reqW = 1 }
	if reqH <= 0 { reqH = 1 }
	// scale down if over cap (preserve aspect)
	scale := 1.0
	if float64(reqW*reqH) > float64(maxPx) {
		scale = math.Sqrt(float64(maxPx) / float64(reqW*reqH))
	}
	effW := int(math.Max(1, math.Round(float64(reqW)*scale)))
	effH := int(math.Max(1, math.Round(float64(reqH)*scale)))
	// also scale iterations a bit to keep time bounded when downscaling is applied
	effIter := params.MaxIter
	if scale < 1.0 {
		effIter = int(math.Max(10, math.Min(float64(maxIt), math.Round(float64(params.MaxIter)*math.Sqrt(scale)))))
	} else if params.MaxIter > maxIt {
		effIter = maxIt
	}
	// use effective sizes for rendering
	effParams := params
	effParams.Width = effW
	effParams.Height = effH
	effParams.MaxIter = effIter

	// Limit concurrency
	select {
	case renderSem <- struct{}{}:
		defer func(){ <-renderSem }()
	case <-time.After(2 * time.Second):
		http.Error(w, "server busy, try again", http.StatusServiceUnavailable)
		return
	}

	// Set an upper bound on render time
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Render PNG
	pngBytes, err := renderPNG(ctx, effParams, randCfg)
	if err != nil {
		if ctx.Err() != nil {
			http.Error(w, "render timeout", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	b64 := base64.StdEncoding.EncodeToString(pngBytes)

	response := map[string]interface{}{
		"pngData":       b64,
		"params":        params,
		"renderWidth":   effW,
		"renderHeight":  effH,
		"usedIterations": effIter,
		"randPalette": randObj,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func renderPNG(ctx context.Context, params FractalParams, randCfg *RandPaletteConfig) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, params.Width, params.Height))
	
	// IFS fractals use point-based rendering (chaos game)
	// Check if this is an IFS fractal type
	if params.FractalType == "fern" || params.FractalType == "sierpinski" || params.FractalType == "dragon" || params.FractalType == "tree" || params.FractalType == "spiral" {
		return renderIFS(ctx, img, params, randCfg)
	}
	
	// Other fractals use per-pixel computation
	stride := img.Stride
	for y := 0; y < params.Height; y++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		for x := 0; x < params.Width; x++ {
			// map to complex plane or coordinate space
			scale := 4.0 / params.Zoom
			var it int
			
			switch params.FractalType {
			case "mandelbrot":
				cx := (float64(x)/float64(params.Width))*scale - scale/2 + params.CenterX
				cy := (float64(y)/float64(params.Height))*scale - scale/2 + params.CenterY
				it = mandelIterFast(cx, cy, params.MaxIter, params.Exponent)
			case "julia":
				zx := (float64(x)/float64(params.Width))*scale - scale/2 + params.CenterX
				zy := (float64(y)/float64(params.Height))*scale - scale/2 + params.CenterY
				it = juliaIteration(zx, zy, params.JuliaCReal, params.JuliaCImag, params.MaxIter, params.Exponent)
			case "newton":
				zx := (float64(x)/float64(params.Width))*scale - scale/2 + params.CenterX
				zy := (float64(y)/float64(params.Height))*scale - scale/2 + params.CenterY
				it = newtonIteration(zx, zy, params.NewtonDegree, params.MaxIter)
			default:
				// Default to Mandelbrot
				cx := (float64(x)/float64(params.Width))*scale - scale/2 + params.CenterX
				cy := (float64(y)/float64(params.Height))*scale - scale/2 + params.CenterY
				it = mandelIterFast(cx, cy, params.MaxIter, params.Exponent)
			}
			
			r, g, b := colorFromIteration(it, params.MaxIter, params.ColorPalette, randCfg)
			off := y*stride + x*4
			img.Pix[off+0] = uint8(r)
			img.Pix[off+1] = uint8(g)
			img.Pix[off+2] = uint8(b)
			img.Pix[off+3] = 255
		}
		if y%16 == 0 { // periodically yield to scheduler to avoid starving system
			runtime.Gosched()
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Faster Mandelbrot iteration with special-case for exponent 2 to avoid trig
func mandelIterFast(cx, cy float64, maxIter int, exp float64) int {
	if exp == 2 {
		var zx, zy float64
		for i := 0; i < maxIter; i++ {
			// z = z^2 + c
			// (x+iy)^2 = (x^2 - y^2) + i(2xy)
			x2 := zx * zx
			y2 := zy * zy
			if x2+y2 > 4.0 {
				return i
			}
			zy = 2*zx*zy + cy
			zx = x2 - y2 + cx
		}
		return maxIter
	}
	// Fallback to general exponent method
	return mandelbrotIteration(cx, cy, maxIter, exp)
}

func generateMandelbrot(params FractalParams, randCfg *RandPaletteConfig) []map[string]int {
	pixels := make([]map[string]int, params.Width*params.Height)
	
	scale := 4.0 / params.Zoom
	xOffset := params.CenterX
	yOffset := params.CenterY

	// choose iterator based on exponent
	useFast := math.Abs(params.Exponent-2.0) < 1e-9

	for y := 0; y < params.Height; y++ {
		for x := 0; x < params.Width; x++ {
			// Map pixel coordinates to complex plane
			cx := (float64(x)/float64(params.Width))*scale - scale/2 + xOffset
			cy := (float64(y)/float64(params.Height))*scale - scale/2 + yOffset

			// Calculate Mandelbrot iteration
			var iter int
			if useFast {
				// fast quadratic iteration without trig
				zx, zy := 0.0, 0.0
				for i := 0; i < params.MaxIter; i++ {
					if zx*zx+zy*zy > 4.0 { iter = i; break }
					nx := zx*zx - zy*zy + cx
					zy = 2.0*zx*zy + cy
					zx = nx
					if i == params.MaxIter-1 { iter = params.MaxIter }
				}
			} else {
				iter = mandelbrotIteration(cx, cy, params.MaxIter, params.Exponent)
			}

			// Color based on iteration count
			r, g, b := colorFromIteration(iter, params.MaxIter, params.ColorPalette, randCfg)

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

func mandelbrotIteration(cx, cy float64, maxIter int, exponent float64) int {
	var zx, zy float64
	for i := 0; i < maxIter; i++ {
		if zx*zx+zy*zy > 4.0 {
			return i
		}
		// Calculate z^n where n is the exponent
		// For z = x + iy, z^n = (x + iy)^n
		// We use polar form: z = r*e^(iθ), so z^n = r^n * e^(inθ)
		// r = sqrt(x^2 + y^2), θ = atan2(y, x)
		r := math.Sqrt(zx*zx + zy*zy)
		if r == 0 {
			zx, zy = cx, cy
			continue
		}
		theta := math.Atan2(zy, zx)
		newR := math.Pow(r, exponent)
		newTheta := theta * exponent
		zx = newR*math.Cos(newTheta) + cx
		zy = newR*math.Sin(newTheta) + cy
	}
	return maxIter
}

// Julia set iteration: z = z^n + c (where c is constant)
func juliaIteration(zx, zy, cr, ci float64, maxIter int, exponent float64) int {
	for i := 0; i < maxIter; i++ {
		if zx*zx+zy*zy > 4.0 {
			return i
		}
		r := math.Sqrt(zx*zx + zy*zy)
		if r == 0 {
			continue
		}
		theta := math.Atan2(zy, zx)
		newR := math.Pow(r, exponent)
		newTheta := theta * exponent
		zx = newR*math.Cos(newTheta) + cr
		zy = newR*math.Sin(newTheta) + ci
	}
	return maxIter
}

// Newton fractal: find roots of z^n - 1 = 0 using Newton's method
func newtonIteration(zx, zy float64, degree int, maxIter int) int {
	// Pre-compute roots of unity: e^(2πik/n) for k=0..n-1
	type root struct{ r, i float64 }
	roots := make([]root, degree)
	for k := 0; k < degree; k++ {
		angle := 2.0 * math.Pi * float64(k) / float64(degree)
		roots[k] = root{math.Cos(angle), math.Sin(angle)}
	}
	
	// Newton's method: z_new = z - f(z)/f'(z)
	// For f(z) = z^n - 1, f'(z) = n*z^(n-1)
	// So: z_new = z - (z^n - 1)/(n*z^(n-1))
	for i := 0; i < maxIter; i++ {
		// Check which root we're closest to
		minDist := 1e10
		closestRoot := -1
		for k := 0; k < degree; k++ {
			dr := zx - roots[k].r
			di := zy - roots[k].i
			dist := dr*dr + di*di
			if dist < minDist {
				minDist = dist
				closestRoot = k
			}
		}
		
		// If very close to a root, return with color based on root
		if minDist < 1e-6 {
			return closestRoot * maxIter / degree
		}
		
		// Compute z^n using polar form
		r := math.Sqrt(zx*zx + zy*zy)
		if r < 1e-10 {
			// Too close to origin, return maxIter
			return maxIter
		}
		theta := math.Atan2(zy, zx)
		
		// z^n = r^n * e^(i*n*theta)
		rn := math.Pow(r, float64(degree))
		nTheta := theta * float64(degree)
		znReal := rn * math.Cos(nTheta)
		znImag := rn * math.Sin(nTheta)
		
		// z^(n-1) = r^(n-1) * e^(i*(n-1)*theta)
		rn1 := math.Pow(r, float64(degree-1))
		n1Theta := theta * float64(degree-1)
		zn1Real := rn1 * math.Cos(n1Theta)
		zn1Imag := rn1 * math.Sin(n1Theta)
		
		// f(z) = z^n - 1
		fReal := znReal - 1.0
		fImag := znImag
		
		// f'(z) = n*z^(n-1)
		fpReal := float64(degree) * zn1Real
		fpImag := float64(degree) * zn1Imag
		
		// f'(z) magnitude squared
		fpMag2 := fpReal*fpReal + fpImag*fpImag
		if fpMag2 < 1e-10 {
			return maxIter
		}
		
		// f(z)/f'(z) = (fReal + i*fImag) / (fpReal + i*fpImag)
		// = (fReal + i*fImag) * (fpReal - i*fpImag) / |f'(z)|^2
		fOverFpReal := (fReal*fpReal + fImag*fpImag) / fpMag2
		fOverFpImag := (fImag*fpReal - fReal*fpImag) / fpMag2
		
		// Newton step: z_new = z - f(z)/f'(z)
		zx = zx - fOverFpReal
		zy = zy - fOverFpImag
		
		// Check for divergence
		if zx*zx+zy*zy > 100 {
			return maxIter
		}
	}
	
	// After maxIter iterations, find closest root for coloring
	minDist := 1e10
	closestRoot := 0
	for k := 0; k < degree; k++ {
		dr := zx - roots[k].r
		di := zy - roots[k].i
		dist := dr*dr + di*di
		if dist < minDist {
			minDist = dist
			closestRoot = k
		}
	}
	return closestRoot * maxIter / degree
}

// IFS (Iterated Function System) - Point-based rendering using chaos game
func renderIFS(ctx context.Context, img *image.RGBA, params FractalParams, randCfg *RandPaletteConfig) ([]byte, error) {
	// Initialize image to black
	for i := range img.Pix {
		img.Pix[i] = 0
	}
	
	// Create a hit counter for each pixel (for density-based coloring)
	width := params.Width
	height := params.Height
	hits := make([][]int, height)
	for i := range hits {
		hits[i] = make([]int, width)
	}
	
	// Number of points to generate (in thousands)
	numPoints := params.IFSPoints * 1000
	
	// Initialize starting point and RNG
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	px, py := 0.0, 0.0
	
	// Transformation functions based on IFS type
	var transform func(float64, float64, *rand.Rand) (float64, float64)
	var bounds struct{ minX, maxX, minY, maxY float64 }
	
	switch params.FractalType {
	case "fern":
		bounds = struct{ minX, maxX, minY, maxY float64 }{-2.5, 2.5, 0, 10}
		transform = func(x, y float64, r *rand.Rand) (float64, float64) {
			rval := r.Float64()
			if rval < 0.01 {
				return 0.0, 0.16 * y
			} else if rval < 0.86 {
				return 0.85*x + 0.04*y, -0.04*x + 0.85*y + 1.6
			} else if rval < 0.93 {
				return 0.20*x - 0.26*y, 0.23*x + 0.22*y + 1.6
			} else {
				return -0.15*x + 0.28*y, 0.26*x + 0.24*y + 0.44
			}
		}
	case "sierpinski":
		bounds = struct{ minX, maxX, minY, maxY float64 }{0, 1, 0, 1}
		// Isosceles triangle: top vertex at center-top, bottom vertices at corners
		// In image coordinates, y=0 is at top, so top vertex at y=0.0, bottom at y=1.0
		vx := []float64{0.5, 0.0, 1.0}  // top center, bottom left, bottom right
		vy := []float64{0.0, 1.0, 1.0}  // top at y=0.0, bottom at y=1.0
		transform = func(x, y float64, r *rand.Rand) (float64, float64) {
			i := r.Intn(3)
			return (x + vx[i]) / 2, (y + vy[i]) / 2
		}
	case "dragon":
		bounds = struct{ minX, maxX, minY, maxY float64 }{0, 1.5, -0.5, 0.5}
		transform = func(x, y float64, r *rand.Rand) (float64, float64) {
			if r.Float64() < 0.5 {
				return 0.5*x - 0.5*y, 0.5*x + 0.5*y
			} else {
				return -0.5*x - 0.5*y + 1, 0.5*x - 0.5*y
			}
		}
	case "tree":
		bounds = struct{ minX, maxX, minY, maxY float64 }{-1, 1, 0, 2}
		transform = func(x, y float64, r *rand.Rand) (float64, float64) {
			rval := r.Float64()
			if rval < 0.1 {
				// Trunk/main stem (vertical, narrow)
				return 0.0, 0.6 * y
			} else if rval < 0.45 {
				// Left branch (rotated left)
				return 0.42*x - 0.42*y, 0.42*x + 0.42*y + 0.6
			} else if rval < 0.8 {
				// Right branch (rotated right)
				return 0.42*x + 0.42*y, -0.42*x + 0.42*y + 0.6
			} else {
				// Small left branch
				return 0.1*x, 0.44*y + 0.6
			}
		}
	case "spiral":
		// Spiral fractal has a wider range, especially in x direction
		bounds = struct{ minX, maxX, minY, maxY float64 }{-8, 3, -1, 3}
		transform = func(x, y float64, r *rand.Rand) (float64, float64) {
			rval := r.Float64()
			if rval < 0.5 {
				return 0.787879*x - 0.424242*y + 1.758647, 0.242424*x + 0.859848*y + 1.408065
			} else {
				return -0.121212*x + 0.257576*y - 6.721654, 0.151515*x + 0.053030*y + 1.377236
			}
		}
	default:
		// Default to fern
		bounds = struct{ minX, maxX, minY, maxY float64 }{-2.5, 2.5, 0, 10}
		transform = func(x, y float64, r *rand.Rand) (float64, float64) {
			rval := r.Float64()
			if rval < 0.01 {
				return 0.0, 0.16 * y
			} else if rval < 0.86 {
				return 0.85*x + 0.04*y, -0.04*x + 0.85*y + 1.6
			} else if rval < 0.93 {
				return 0.20*x - 0.26*y, 0.23*x + 0.22*y + 1.6
			} else {
				return -0.15*x + 0.28*y, 0.26*x + 0.24*y + 0.44
			}
		}
	}
	
	// Calculate view bounds with zoom and center
	baseWidth := bounds.maxX - bounds.minX
	baseHeight := bounds.maxY - bounds.minY
	viewWidth := baseWidth / params.Zoom
	viewHeight := baseHeight / params.Zoom
	centerX := (bounds.minX + bounds.maxX) / 2
	centerY := (bounds.minY + bounds.maxY) / 2
	viewMinX := centerX + params.CenterX - viewWidth/2
	viewMaxX := centerX + params.CenterX + viewWidth/2
	viewMinY := centerY + params.CenterY - viewHeight/2
	viewMaxY := centerY + params.CenterY + viewHeight/2
	
	// For dragon curve, use line-based rendering instead of point-based
	if params.FractalType == "dragon" {
		// Generate dragon curve using L-system approach (iterative line replacement)
		// Start with a simple line segment and iteratively replace with dragon curve segments
		iterations := 18 // Number of iterations for detailed curve (increased by 50%)
		points := generateDragonCurve(iterations)
		
		// Draw lines between consecutive points
		stride := img.Stride
		var r, g, b int
		if params.ColorPalette == "white" {
			r, g, b = 255, 255, 255
		} else {
			// Use a default color for lines
			r, g, b = colorFromIteration(params.MaxIter/2, params.MaxIter, params.ColorPalette, randCfg)
		}
		
		for i := 0; i < len(points)-1; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			
			// Apply shift to points
			x1 := points[i].X + 0.50
			y1 := points[i].Y - 0.10
			x2 := points[i+1].X + 0.50
			y2 := points[i+1].Y - 0.10
			
			// Map to image coordinates
			normX1 := (x1 - viewMinX) / (viewMaxX - viewMinX)
			normY1 := (y1 - viewMinY) / (viewMaxY - viewMinY)
			normX2 := (x2 - viewMinX) / (viewMaxX - viewMinX)
			normY2 := (y2 - viewMinY) / (viewMaxY - viewMinY)
			
			imgX1 := int(normX1 * float64(width))
			imgY1 := int(normY1 * float64(height))
			imgX2 := int(normX2 * float64(width))
			imgY2 := int(normY2 * float64(height))
			
			// Draw line using Bresenham's algorithm
			drawLine(img, imgX1, imgY1, imgX2, imgY2, r, g, b, stride)
			
			if i%1000 == 0 {
				runtime.Gosched()
			}
		}
	} else {
		// Generate points using chaos game for other IFS types
		maxHits := 0
		for i := 0; i < numPoints; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			
			// Apply transformation
			px, py = transform(px, py, rng)
			
			// Map to image coordinates using view bounds
			normX := (px - viewMinX) / (viewMaxX - viewMinX)
			normY := (py - viewMinY) / (viewMaxY - viewMinY)
			
			imgX := int(normX * float64(width))
			imgY := int(normY * float64(height))
			
			if imgX >= 0 && imgX < width && imgY >= 0 && imgY < height {
				hits[imgY][imgX]++
				if hits[imgY][imgX] > maxHits {
					maxHits = hits[imgY][imgX]
				}
			}
			
			if i%10000 == 0 {
				runtime.Gosched()
			}
		}
	}
	
	// Render hits to image with coloring (only for non-dragon IFS types)
	if params.FractalType != "dragon" {
		stride := img.Stride
		maxHits := 0
		// Find max hits
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if hits[y][x] > maxHits {
					maxHits = hits[y][x]
				}
			}
		}
		
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if hits[y][x] > 0 {
					var r, g, b int
					// For IFS fractals with white palette, use solid white instead of gradient
					if params.ColorPalette == "white" {
						r, g, b = 255, 255, 255
					} else {
						// Use hit count for coloring (density-based)
						hitRatio := float64(hits[y][x]) / float64(maxHits)
						it := int(hitRatio * float64(params.MaxIter))
						r, g, b = colorFromIteration(it, params.MaxIter, params.ColorPalette, randCfg)
					}
					off := y*stride + x*4
					img.Pix[off+0] = uint8(r)
					img.Pix[off+1] = uint8(g)
					img.Pix[off+2] = uint8(b)
					img.Pix[off+3] = 255
				}
			}
		}
	}
	
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Helper function to draw text labels on image
func drawLabel(img *image.RGBA, text string, x, y int, col color.RGBA) {
	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(text)
}

// Generate dragon curve points using L-system approach
func generateDragonCurve(iterations int) []struct{ X, Y float64 } {
	// Start with a simple line segment from (0,0) to (1,0)
	points := []struct{ X, Y float64 }{
		{0.0, 0.0},
		{1.0, 0.0},
	}
	
	// Iteratively replace each segment with two segments forming a right angle
	// Dragon curve: each segment is replaced by two segments at 90 degrees
	// Direction alternates: left, right, left, right, etc.
	for iter := 0; iter < iterations; iter++ {
		newPoints := make([]struct{ X, Y float64 }, 0, len(points)*2)
		newPoints = append(newPoints, points[0]) // Keep first point
		
		for i := 0; i < len(points)-1; i++ {
			x1, y1 := points[i].X, points[i].Y
			x2, y2 := points[i+1].X, points[i+1].Y
			
			// Calculate midpoint
			midX := (x1 + x2) / 2.0
			midY := (y1 + y2) / 2.0
			
			// Calculate perpendicular point (rotate 90 degrees around midpoint)
			dx := x2 - x1
			dy := y2 - y1
			
			// Rotate vector 90 degrees and scale by 0.5
			// For dragon curve, alternate rotation direction based on segment index
			// Use bit manipulation to determine direction
			rotX := -dy * 0.5
			rotY := dx * 0.5
			// Count set bits in i to determine direction (alternating pattern)
			count := 0
			for n := i; n > 0; n >>= 1 {
				count += n & 1
			}
			if count%2 == 1 {
				rotX = dy * 0.5
				rotY = -dx * 0.5
			}
			
			// Add midpoint + rotation
			newPoints = append(newPoints, struct{ X, Y float64 }{midX + rotX, midY + rotY})
			newPoints = append(newPoints, points[i+1]) // Keep endpoint
		}
		
		points = newPoints
	}
	
	return points
}

// Draw a line using Bresenham's algorithm
func drawLine(img *image.RGBA, x1, y1, x2, y2, r, g, b int, stride int) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	
	// Clamp coordinates to image bounds
	if x1 < 0 { x1 = 0 }
	if x1 >= width { x1 = width - 1 }
	if y1 < 0 { y1 = 0 }
	if y1 >= height { y1 = height - 1 }
	if x2 < 0 { x2 = 0 }
	if x2 >= width { x2 = width - 1 }
	if y2 < 0 { y2 = 0 }
	if y2 >= height { y2 = height - 1 }
	
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}
	err := dx - dy
	
	x, y := x1, y1
	for {
		off := y*stride + x*4
		img.Pix[off+0] = uint8(r)
		img.Pix[off+1] = uint8(g)
		img.Pix[off+2] = uint8(b)
		img.Pix[off+3] = 255
		
		if x == x2 && y == y2 {
			break
		}
		
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Helper: clamp integer to 0..255
func clamp255(v int) int {
    if v < 0 { return 0 }
    if v > 255 { return 255 }
    return v
}

// Helper: linear interpolation
func lerp(a, b float64, t float64) float64 { return a + (b-a)*t }

// Helper: HSV to RGB conversion. h in [0,360), s,v in [0,1]
func hsvToRgb(h, s, v float64) (int, int, int) {
    c := v * s
    x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
    m := v - c
    var r1, g1, b1 float64
    switch {
    case h < 60:
        r1, g1, b1 = c, x, 0
    case h < 120:
        r1, g1, b1 = x, c, 0
    case h < 180:
        r1, g1, b1 = 0, c, x
    case h < 240:
        r1, g1, b1 = 0, x, c
    case h < 300:
        r1, g1, b1 = x, 0, c
    default:
        r1, g1, b1 = c, 0, x
    }
    r := clamp255(int(math.Round((r1+m) * 255)))
    g := clamp255(int(math.Round((g1+m) * 255)))
    b := clamp255(int(math.Round((b1+m) * 255)))
    return r, g, b
}

// Compute a color from a 24-step palette index using HSV ramps
func colorFromIteration(iter, maxIter int, palette string, randCfg *RandPaletteConfig) (int, int, int) {
    if iter == maxIter {
        return 0, 0, 0 // Black for points in the set
    }
    // Continuous palette mapping across the entire spectrum
    t := float64(iter) / float64(maxIter) // 0..1 (no reverse)
    switch palette {
    case "random":
        if randCfg != nil {
            h := math.Mod(randCfg.baseHue+randCfg.hueSpan*t, 360.0)
            s := lerp(randCfg.satMin, randCfg.satMax, t)
            v := lerp(randCfg.valMin, randCfg.valMax, t)
            return hsvToRgb(h, s, v)
        }
        // fallback full spectrum
        return hsvToRgb(360.0*t, 1.0, 1.0)
    case "classic":
        return hsvToRgb(360.0*t, 1.0, 1.0)
    case "white":
        // White palette: white for points outside set, black for inside
        // Use brightness based on iteration count
        brightness := int(255 * t)
        return brightness, brightness, brightness
    default:
        // default to full spectrum
        return hsvToRgb(360.0*t, 1.0, 1.0)
    }
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

// Generate a small Mandelbrot fractal favicon
func generateFavicon() ([]byte, error) {
	size := 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	
	// Center and zoom for an interesting part of the Mandelbrot set
	centerX := -0.5
	centerY := 0.0
	zoom := 1.0
	maxIter := 50
	scale := 4.0 / zoom
	
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Map pixel to complex plane
			cx := (float64(x)/float64(size))*scale - scale/2 + centerX
			cy := (float64(y)/float64(size))*scale - scale/2 + centerY
			
			// Calculate Mandelbrot iteration
			zx, zy := 0.0, 0.0
			iter := 0
			for iter < maxIter {
				if zx*zx+zy*zy > 4.0 {
					break
				}
				x2 := zx * zx
				y2 := zy * zy
				zy = 2*zx*zy + cy
				zx = x2 - y2 + cx
				iter++
			}
			
			// Color based on iteration count (vibrant colors)
			var r, g, b int
			if iter >= maxIter {
				r, g, b = 0, 0, 0 // Black for inside set
			} else {
				t := float64(iter) / float64(maxIter)
				// Use HSV to RGB for vibrant colors
				h := 360.0 * t
				r, g, b = hsvToRgb(h, 1.0, 1.0)
			}
			
			offset := y*img.Stride + x*4
			img.Pix[offset+0] = uint8(r)
			img.Pix[offset+1] = uint8(g)
			img.Pix[offset+2] = uint8(b)
			img.Pix[offset+3] = 255
		}
	}
	
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func handleFavicon(w http.ResponseWriter, r *http.Request) {
	favicon, err := generateFavicon()
	if err != nil {
		http.Error(w, "Error generating favicon", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day
	w.Write(favicon)
}

