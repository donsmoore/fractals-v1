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
	"bytes"
	"context"
	"runtime"
)

type FractalParams struct {
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	CenterX float64 `json:"centerX"`
	CenterY float64 `json:"centerY"`
	Zoom    float64 `json:"zoom"`
	MaxIter int     `json:"maxIter"`
	Exponent float64 `json:"exponent"` // Power for z^n + c (default 2.0 for standard Mandelbrot)
	ColorPalette string `json:"colorPalette"` // Color palette name: "classic", "fire", "ocean", "sunset"
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
            white-space: nowrap;
        }
        .control-group {
            display: flex;
            gap: 5px;
            align-items: center;
            margin-left: 10px;
        }
        .control-group label {
            color: #e0e0e0;
            font-size: 14px;
            white-space: nowrap;
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
                <div id="infoOverlay" class="info-overlay">Center: (0.0000, 0.0000) | Zoom: 1.00x</div>
                <div class="control-group">
                <label for="exponentInput">Exponent (z^n + c):</label>
                <input type="number" id="exponentInput" step="0.1" min="1" max="10" value="2.0" onkeypress="if(event.key==='Enter') updateExponent()">
                <button onclick="updateExponent()">Update Exponent</button>
                </div>
                <div class="control-group">
                <label for="paletteSelect">Color Palette:</label>
                <select id="paletteSelect" onchange="updatePalette()" style="padding: 8px; border: 1px solid #555; border-radius: 4px; background: #333; color: #e0e0e0; cursor: pointer;">
                    <option value="classic">Classic</option>
                    <option value="random">Random</option>
                </select>
                </div>
                <div class="control-group">
                    <label for="qualitySelect">Quality:</label>
                    <select id="qualitySelect" onchange="updateQuality()" style="padding: 6px; border: 1px solid #555; border-radius: 4px; background: #333; color: #e0e0e0; min-width: 120px; max-width: 140px;">
                        <option value="low">Low</option>
                        <option value="medium" selected>Medium</option>
                        <option value="high">High</option>
                        <option value="auto">Auto</option>
                    </select>
                </div>
            </div>
            <div class="controls-right">
                <button onclick="generateFractal()">Generate Fractal</button>
                <button onclick="resetView()">Reset View</button>
                <button onclick="printFractal()">Print</button>
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
            exponent: 2.0,  // Default exponent for standard Mandelbrot (z^2 + c)
            colorPalette: 'classic'  // Default color palette
        };
        
        // Initialize the input field with default value
        document.addEventListener('DOMContentLoaded', function() {
            document.getElementById('exponentInput').value = params.exponent;
            document.getElementById('paletteSelect').value = params.colorPalette;
            document.getElementById('qualitySelect').value = quality; // Initialize quality
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
            el.textContent = 'Center: (' + fmt(params.centerX) + ', ' + fmt(params.centerY) + ') | Zoom: ' + params.zoom.toFixed(2) + 'x';
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
                exponent: params.exponent,
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
                try { lastRenderedImageData = ctx.getImageData(0, 0, canvas.width, canvas.height); } catch (e) { lastRenderedImageData = null; }
            };
            img.src = 'data:image/png;base64,' + data.pngData;
        }

        function resetView() {
            params.centerX = 0.0;
            params.centerY = 0.0;
            params.zoom = 1.0;
            params.exponent = 2.0;
            params.colorPalette = 'classic';
            document.getElementById('exponentInput').value = params.exponent;
            document.getElementById('paletteSelect').value = params.colorPalette;
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
        
        function updatePalette() {
            const paletteSelect = document.getElementById('paletteSelect');
            params.colorPalette = paletteSelect.value;
            generateFractal();
        }
 
         function printFractal() {
            // Generate a true 4K image client-side with current view parameters
            const exportW = 3840, exportH = 2160;
            const cx = params.centerX;
            const cy = params.centerY;
            const zoom = params.zoom;
            const maxIter = params.maxIter;
            const exponent = params.exponent || 2.0;
            const palette = params.colorPalette || 'classic';

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
                // Setup progress elements
                var fillEl=doc.getElementById('fill');
                var pctEl=doc.getElementById('pct');

                // Create worker code
                var workerSrc='';
                workerSrc+='self.onmessage=function(e){';
                workerSrc+='var d=e.data,W=d.W,H=d.H,cx=d.cx,cy=d.cy,zoom=d.zoom,maxIter=d.maxIter,exp=d.exp,palette=d.palette,randCfg=d.randCfg;';
                workerSrc+='function c255(v){return v<0?0:(v>255?255:v);}';
                workerSrc+='function lrp(a,b,t){return a+(b-a)*t;}';
                workerSrc+='function hsv2rgb(h,s,v){var c=v*s;var x=c*(1-Math.abs(((h/60)%2)-1));var m=v-c;var r1=0,g1=0,b1=0;';
                workerSrc+='if(h<60){r1=c;g1=x;b1=0;} else if(h<120){r1=x;g1=c;b1=0;} else if(h<180){r1=0;g1=c;b1=x;} else if(h<240){r1=0;g1=x;b1=c;} else if(h<300){r1=x;g1=0;b1=c;} else {r1=c;g1=0;b1=x;}';
                workerSrc+='return [c255(Math.round((r1+m)*255)),c255(Math.round((g1+m)*255)),c255(Math.round((b1+m)*255))];}';
                workerSrc+='function colorFromIter(it,mi){if(it===mi){return [0,0,0];} var t=it/mi; if(palette==="random" && randCfg){var h=(randCfg.baseHue+randCfg.hueSpan*t)%360; var s=lrp(randCfg.satMin,randCfg.satMax,t); var v=lrp(randCfg.valMin,randCfg.valMax,t); return hsv2rgb(h,s,v);} return hsv2rgb(360*t,1,1);}';
                workerSrc+='function mandel(cx,cy,mi,ex){var zx=0,zy=0; for(var i=0;i<mi;i++){if(zx*zx+zy*zy>4.0){return i;} var r=Math.hypot(zx,zy); if(r===0){zx=cx;zy=cy;continue;} var th=Math.atan2(zy,zx); var nr=Math.pow(r,ex); var nth=th*ex; zx=nr*Math.cos(nth)+cx; zy=nr*Math.sin(nth)+cy;} return mi;}';
                workerSrc+='var buf=new Uint8ClampedArray(W*H*4); var scale=4.0/zoom; var chunk=24; for(var y=0;y<H;y++){ var ny=y/H; for(var x=0;x<W;x++){ var nx=x/W; var cxp=nx*scale - scale/2 + cx; var cyp=ny*scale - scale/2 + cy; var it=mandel(cxp,cyp,maxIter,exp); var rgb=colorFromIter(it,maxIter); var idx=(y*W+x)*4; buf[idx]=rgb[0]; buf[idx+1]=rgb[1]; buf[idx+2]=rgb[2]; buf[idx+3]=255;} if(y%chunk===chunk-1){ self.postMessage({type:"progress", y:y+1, H:H}); } } self.postMessage({type:"done", buffer:buf.buffer}, [buf.buffer]);';
                workerSrc+='}';

                var blob=new doc.defaultView.Blob([workerSrc],{type:'application/javascript'});
                var url=doc.defaultView.URL.createObjectURL(blob);
                var WorkerCtor=doc.defaultView.Worker;
                var worker=new WorkerCtor(url);
                // random palette config
                var rnd=null; if(palette==='random'){ rnd={ baseHue: Math.random()*360, hueSpan: 180+Math.random()*180, satMin:0.6, satMax:1.0, valMin:0.8, valMax:1.0}; }
                worker.onmessage=function(ev){ var msg=ev.data; if(msg.type==='progress'){ var p=Math.floor((msg.y/msg.H)*100); if(fillEl){fillEl.style.width=p+'%';} if(pctEl){pctEl.textContent=p+'%';} } else if(msg.type==='done'){ var off=doc.createElement('canvas'); off.width=exportW; off.height=exportH; var octx=off.getContext('2d'); var arr=new Uint8ClampedArray(msg.buffer); var img=octx.createImageData(exportW,exportH); img.data.set(arr); octx.putImageData(img,0,0); var dataUrl=off.toDataURL('image/png'); doc.open(); doc.write('<!DOCTYPE html><html><head><title>Fractal 4K Export</title><style>html,body{margin:0;height:100%;background:#000;} .toolbar{position:fixed;top:10px;right:10px;z-index:5} .btn{padding:8px 12px;border:1px solid #555;border-radius:4px;background:#333;color:#e0e0e0;text-decoration:none} img{display:block;width:100vw;height:100vh;object-fit:contain;background:#000;}</style></head><body>'); doc.write('<div class="toolbar"><a id="dl" class="btn" download="fractal-4k.png" href="'+dataUrl+'">Download PNG (4K)</a></div>'); doc.write('<img src="'+dataUrl+'" alt="Fractal 4K"/>'); doc.write('<scr'+'ipt>window.addEventListener("load",function(){var a=document.getElementById("dl"); if(a){try{a.click();}catch(e){}}});</scr'+'ipt>'); doc.write('</body></html>'); doc.close(); worker.terminate(); doc.defaultView.URL.revokeObjectURL(url);} };
                worker.postMessage({ W:exportW, H:exportH, cx:cx, cy:cy, zoom:zoom, maxIter:maxIter, exp:exponent, palette:palette, randCfg:rnd });
            })(w.document);
         }

        function updateQuality(){
            // apply new quality immediately
            generateFractal();
        }

        // Map normalized canvas coordinates to complex plane at current view
        function normToComplex(nx, ny) {
            const scale = 4.0 / params.zoom;
            const cx = nx * scale - scale / 2 + params.centerX;
            const cy = ny * scale - scale / 2 + params.centerY;
            return { cx, cy };
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
            const center = normToComplex(cxn, cyn);
            params.centerX = center.cx;
            params.centerY = center.cy;
            params.zoom = Math.max(0.0001, params.zoom / widthNorm);
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
	// Set default color palette
	if params.ColorPalette == "" {
		params.ColorPalette = "classic"
	}

	// Optional random palette configuration for this request
	var randCfg *RandPaletteConfig
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
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func renderPNG(ctx context.Context, params FractalParams, randCfg *RandPaletteConfig) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, params.Width, params.Height))
	// generate into RGBA buffer
	stride := img.Stride
	// compute once per request palette behavior (random handled already in colorFromIteration via randCfg)
	for y := 0; y < params.Height; y++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		ny := y
		_ = ny
		for x := 0; x < params.Width; x++ {
			// compute color via existing pipeline
			// map to complex plane
			scale := 4.0 / params.Zoom
			cx := (float64(x)/float64(params.Width))*scale - scale/2 + params.CenterX
			cy := (float64(y)/float64(params.Height))*scale - scale/2 + params.CenterY
			it := mandelIterFast(cx, cy, params.MaxIter, params.Exponent)
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

