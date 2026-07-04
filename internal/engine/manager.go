package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"github.com/user/VLX_VisionBridge/internal/db"
	"github.com/user/VLX_VisionBridge/internal/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type tailBuffer struct {
	buf []byte
	mu  sync.Mutex
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	const maxLen = 4096
	t.buf = append(t.buf, p...)
	if len(t.buf) > maxLen {
		t.buf = t.buf[len(t.buf)-maxLen:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

type RetryTracker struct {
	ConsecutiveCrashes int
	LastCrash          time.Time
}

type ProcessManager struct {
	cmd           *exec.Cmd
	config        *models.Config
	db            *sql.DB
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	cond          *sync.Cond
	isRunning     bool
	overlayCmds   map[int]*exec.Cmd
	retries       map[string]*RetryTracker
	wsClients     map[*websocket.Conn]bool
	wsMutex       sync.Mutex
	overlayServer *http.Server
}

func NewProcessManager(dbConn *sql.DB) *ProcessManager {
	pm := &ProcessManager{
		db:          dbConn,
		overlayCmds: make(map[int]*exec.Cmd),
		retries:     make(map[string]*RetryTracker),
		wsClients:   make(map[*websocket.Conn]bool),
	}
	pm.cond = sync.NewCond(&pm.mu)
	return pm
}

// startOverlayServer initializes the dynamic HTTP/WebSocket server based on configuration parameters.
func (pm *ProcessManager) startOverlayServer(cfg *models.Config) {
	pm.mu.Lock()
	if pm.overlayServer != nil {
		_ = pm.overlayServer.Shutdown(context.Background())
		pm.overlayServer = nil
	}
	pm.mu.Unlock()

	if !cfg.Input.OverlayServerActive {
		log.Println("Overlay Web Server is disabled in configuration. Falling back to strict file:// protocols.")
		return
	}

	port := cfg.Input.OverlayServerPort
	if port <= 0 {
		port = 50051
	}

	mediaPath := cfg.Input.MediaFolderPath
	if mediaPath == "" {
		mediaPath = "/opt/VLX_VisionBridge/media"
	}

	mux := http.NewServeMux()

	// 1. HOSTING HTML OVERLAY
	mux.HandleFunc("/overlay", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, "/opt/VLX_VisionBridge/var/overlay.html")
	})

	// 2. HOSTING SECURE MEDIA DIRECTORY
	mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(mediaPath))))

	// 3. WEBSOCKET CONTROL PLANE
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade protocol failed: %v", err)
			return
		}

		pm.wsMutex.Lock()
		if pm.wsClients == nil {
			pm.wsClients = make(map[*websocket.Conn]bool)
		}
		pm.wsClients[c] = true
		pm.wsMutex.Unlock()

		defer func() {
			pm.wsMutex.Lock()
			delete(pm.wsClients, c)
			pm.wsMutex.Unlock()
			c.Close()
		}()

		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				break
			}

			var req map[string]string
			if json.Unmarshal(msg, &req) == nil && req["action"] == "hello" {
				pm.mu.Lock()
				currentCfg := pm.config
				pm.mu.Unlock()
				if currentCfg != nil {
					syncMsg := pm.buildSyncMessage(currentCfg)
					pm.wsMutex.Lock()
					_ = c.WriteJSON(syncMsg)
					pm.wsMutex.Unlock()
				}
			}
		}
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	pm.mu.Lock()
	pm.overlayServer = srv
	pm.mu.Unlock()

	go func() {
		log.Printf("Starting internal HTTP/WebSocket Control Server on %s (Media Root: %s)", srv.Addr, mediaPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Internal Server initialization failed: %v", err)
		}
	}()
}

func (pm *ProcessManager) Start(ctx context.Context, config *models.Config) error {
	pm.mu.Lock()
	if pm.isRunning {
		pm.mu.Unlock()
		return fmt.Errorf("process manager is already running")
	}

	pm.config = config
	pm.ctx, pm.cancel = context.WithCancel(ctx)
	pm.isRunning = true
	pm.mu.Unlock()

	pm.startOverlayServer(config)
	go pm.monitor()
	go pm.StartConnectorListener()

	return nil
}

func (pm *ProcessManager) Stop() {
	pm.mu.Lock()
	if !pm.isRunning {
		pm.mu.Unlock()
		return
	}
	pm.isRunning = false

	if pm.overlayServer != nil {
		_ = pm.overlayServer.Shutdown(context.Background())
	}

	if pm.cmd != nil && pm.cmd.Process != nil {
		log.Println("Signaling GStreamer pipeline to terminate gracefully...")
		_ = pm.cmd.Process.Signal(syscall.SIGTERM)
	}

	if pm.cancel != nil {
		pm.cancel()
	}
	if pm.cond != nil {
		pm.cond.Broadcast()
	}

	for id, overlayCmd := range pm.overlayCmds {
		if overlayCmd != nil && overlayCmd.Process != nil {
			log.Printf("Signaling background process %d to terminate...", id)
			_ = overlayCmd.Process.Signal(syscall.SIGTERM)
		}
	}

	pm.mu.Unlock()
}

func (pm *ProcessManager) ResolvePath(basePath string) []string {
	if basePath == "" {
		return nil
	}
	if strings.HasPrefix(basePath, "http://") || strings.HasPrefix(basePath, "https://") {
		return []string{basePath}
	}

	info, err := os.Stat(basePath)
	if err != nil || !info.IsDir() {
		return []string{basePath}
	}

	var files []string
	entries, err := os.ReadDir(basePath)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				lower := strings.ToLower(e.Name())
				if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") ||
					strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
					strings.HasSuffix(lower, ".html") {
					files = append(files, filepath.Join(basePath, e.Name()))
				}
			}
		}
	}
	return files
}

func buildLayerStyle(zIndex int, width, height, x, y *int) string {
	style := fmt.Sprintf("z-index: %d; position: absolute; ", zIndex)
	if x != nil {
		style += fmt.Sprintf("left: %dpx; ", *x)
	} else {
		style += "left: 0; "
	}
	if y != nil {
		style += fmt.Sprintf("top: %dpx; ", *y)
	} else {
		style += "top: 0; "
	}
	if width != nil {
		style += fmt.Sprintf("width: %dpx; ", *width)
	} else {
		style += "width: 100%%; "
	}
	if height != nil {
		style += fmt.Sprintf("height: %dpx; ", *height)
	} else {
		style += "height: 100%%; "
	}
	return style
}

func (pm *ProcessManager) startEnvironment(resWidth, resHeight string) {
	if _, exists := pm.overlayCmds[100]; !exists {
		xvfbCmd := exec.Command("Xvfb", ":99", "-screen", "0", fmt.Sprintf("%sx%sx24", resWidth, resHeight), "-ac", "-nolisten", "tcp")
		if err := xvfbCmd.Start(); err != nil {
			log.Printf("Xvfb daemon may already be running: %v", err)
		} else {
			log.Printf("Initialized Xvfb on display :99 (%sx%s)", resWidth, resHeight)
			pm.overlayCmds[100] = xvfbCmd
			time.Sleep(1 * time.Second)
		}
	}

	os.MkdirAll("/tmp/pulse-visionbridge", 0755)
	pulseCmd := exec.Command("pulseaudio", 
		"-D", 
		"--exit-idle-time=-1", 
		"-n", 
		"--load=module-native-protocol-unix auth-anonymous=1 socket=/tmp/pulse-visionbridge/native",
		"--load=module-null-sink sink_name=VisionBridgeSink",
	)
	
	if err := pulseCmd.Run(); err != nil {
		log.Printf("Isolated PulseAudio instance is currently active.")
	} else {
		log.Println("Isolated PulseAudio instance successfully initialized for software loopback.")
	}
}

func (pm *ProcessManager) manageOverlays(cfg *models.Config) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	activeOverlays := make(map[int]bool)
	activeOverlays[100] = true // Preserve Xvfb execution state

	if cfg.Input.ChromiumSource.Active {
		activeOverlays[99] = true

		shouldStart := true
		if cmd, exists := pm.overlayCmds[99]; exists && cmd != nil && cmd.ProcessState == nil {
			shouldStart = false
		}

		if shouldStart {
			bgColor := cfg.Input.BgColor
			if bgColor == "" {
				bgColor = "black"
			}

			carouselDelayMs := 5000
			if cfg.Input.CarouselDelay > 0 {
				carouselDelayMs = cfg.Input.CarouselDelay * 1000
			}

			overlayPort := cfg.Input.OverlayServerPort
			if overlayPort <= 0 {
				overlayPort = 50051
			}

			mediaBasePath := cfg.Input.MediaFolderPath
			if mediaBasePath == "" {
				mediaBasePath = "/opt/VLX_VisionBridge/media"
			}

			resParts := strings.Split(cfg.Input.Resolution, "x")
			resWidth := "1920"
			resHeight := "1080"
			if len(resParts) == 2 {
				resWidth = resParts[0]
				resHeight = resParts[1]
			}

			pm.startEnvironment(resWidth, resHeight)

webrtcHtmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>WebRTC Bridge</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body, html { 
            width: 100vw; 
            height: 100vh; 
            overflow: hidden; 
            background: %s; /* Iniettiamo il bgColor (es. "black") */
            display: flex;
            justify-content: center;
            align-items: center;
        }
        video { 
            width: 100%%; 
            height: 100%%; 
            object-fit: contain; /* Rispetta le proporzioni originali */
            border: none; 
            outline: none; 
        }
    </style>
</head>
<body>
    <video id="live-stream" autoplay playsinline></video>

    <script>
        const videoElement = document.getElementById('live-stream');

        const urlParams = new URLSearchParams(window.location.search);
        const streamName = urlParams.get('stream') || 'default_stream';
        const streamUser = urlParams.get('user') || 'visionbridge';
        const streamPass = urlParams.get('pass') || 'visionbridge';

		const whepUrl = 'http://127.0.0.1:8889/' + streamName + '/whep';

	async function initializeWebRTC() {
			const peerConnection = new RTCPeerConnection();

			peerConnection.addTransceiver('video', { direction: 'recvonly' });
			peerConnection.addTransceiver('audio', { direction: 'recvonly' });

			// FIX 1: Gestione flessibile delle tracce, previene il crash se event.streams[0] non esiste
			peerConnection.ontrack = (event) => {
				if (event.streams && event.streams.length > 0) {
					videoElement.srcObject = event.streams[0];
				} else {
					let stream = videoElement.srcObject;
					if (!stream) {
						stream = new MediaStream();
					}
					stream.addTrack(event.track);
					videoElement.srcObject = stream; 
				}
			};

			const offer = await peerConnection.createOffer();
			await peerConnection.setLocalDescription(offer);

			// FIX 2: Mette in pausa l'esecuzione finché il browser non ha raccolto tutti i candidati ICE locali
			await new Promise((resolve) => {
				if (peerConnection.iceGatheringState === 'complete') {
					resolve();
				} else {
					const checkState = () => {
						if (peerConnection.iceGatheringState === 'complete') {
							peerConnection.removeEventListener('icegatheringstatechange', checkState);
							resolve();
						}
					};
					peerConnection.addEventListener('icegatheringstatechange', checkState);
				}
			});

			const response = await fetch(whepUrl, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/sdp',
					'Authorization': 'Basic ' + btoa(streamUser + ":" + streamPass)
				},
				// Invia a MediaMTX l'SDP FINALE arricchito con gli indirizzi ICE validi
				body: peerConnection.localDescription.sdp
			});

			if (!response.ok) {
				console.error("Failed to connect to MediaMTX WebRTC API");
				setTimeout(initializeWebRTC, 2000);
				return;
			}

			const answerSdp = await response.text();
			await peerConnection.setRemoteDescription(
				new RTCSessionDescription({ type: 'answer', sdp: answerSdp })
			);
		}


        initializeWebRTC().catch(console.error);
    </script>
</body>
</html>`, bgColor)
if err := os.MkdirAll(mediaBasePath, 0755); err == nil {
				webrtcFilePath := filepath.Join(mediaBasePath, "webrtc.html")
				os.WriteFile(webrtcFilePath, []byte(webrtcHtmlContent), 0644)
			}

			htmlContent := `<!DOCTYPE html>
<html>
<head>
<title>VisionBridge</title>
<style>
  * { margin: 0; padding: 0; overflow: hidden; box-sizing: border-box; }
  body { margin: 0; padding: 0; overflow: hidden; background: ` + bgColor + `; width: 100vw; height: 100vh; position: relative; }
</style>
</head>
<body>
  <div id="z1"></div><div id="z2"></div><div id="z3"></div><div id="z4"></div>
  <div id="z5"></div><div id="z6"></div><div id="z7"></div><div id="z8"></div><div id="z9"></div>

  <script>
    var carousels = {};
    var serverPort = ` + strconv.Itoa(overlayPort) + `;
    var mediaBasePath = "` + mediaBasePath + `";

    function createMediaElement(path, volume) {
        var lower = path.toLowerCase();
        var isVideo = lower.endsWith('.mp4') || lower.endsWith('.webm');
        var isImg = lower.endsWith('.png') || lower.endsWith('.jpg') || lower.endsWith('.jpeg');
        var el;

        if (isVideo) {
            el = document.createElement('video');
            el.autoplay = true;
            el.playsInline = true;
        } else if (isImg) {
            el = document.createElement('img');
        } else {
            el = document.createElement('iframe');
            el.allow = "autoplay; camera; microphone; display-capture";
            el.setAttribute("allowtransparency", "true");
            el.frameBorder = "0";
        }

        if (volume !== undefined && volume !== null) {
            var v = parseFloat(volume);
            if (v === 0) {
                el.muted = true;
            } else {
                el.muted = false;
                el.volume = v / 100.0;
            }
        }
        
        // Smart URL Resolution: Dynamically routes requested files through the local HTTP server
        if (path.startsWith('http://') || path.startsWith('https://')) {
            el.src = path;
        } else {
            if (path.startsWith(mediaBasePath)) {
                var relativePath = path.substring(mediaBasePath.length);
                if (relativePath.startsWith('/')) relativePath = relativePath.substring(1);
                el.src = 'http://127.0.0.1:' + serverPort + '/media/' + encodeURI(relativePath);
            } else {
                // Fallback
                el.src = 'file://' + encodeURI(path);
            }
        }

        el.style.width = '100%';
        el.style.height = '100%';
        el.style.border = 'none';
        el.style.objectFit = 'fill';
        el.style.position = 'absolute';
        
        return el;
    }

    function stopLayer(layerId) {
        var container = document.getElementById(layerId);
        if (!container) return;
        container.style.display = 'none';
        container.innerHTML = '';
        if (carousels[layerId]) {
            clearTimeout(carousels[layerId].timer);
            delete carousels[layerId];
        }
    }

    function playLayer(layerId, files, volume) {
        stopLayer(layerId);
        var container = document.getElementById(layerId);
        if (!container || !files || files.length === 0) return;
        container.style.display = 'block';

        if (files.length === 1) {
            var el = createMediaElement(files[0], volume);
            if (el.tagName === 'VIDEO') el.loop = true;
            container.appendChild(el);
        } else {
            carousels[layerId] = { files: files, index: 0, timer: null, volume: volume };
            playNextCarousel(layerId);
        }
    }

    function playNextCarousel(layerId) {
        var c = carousels[layerId];
        if (!c) return;
        var container = document.getElementById(layerId);
        container.innerHTML = ''; 

        var path = c.files[c.index];
        var el = createMediaElement(path, c.volume);
        container.appendChild(el);

        if (el.tagName === 'VIDEO') {
            el.onended = function() {
                setTimeout(function() {
                    c.index = (c.index + 1) % c.files.length;
                    playNextCarousel(layerId);
                }, ` + strconv.Itoa(carouselDelayMs) + `);
            };
        } else {
            c.timer = setTimeout(function() {
                c.index = (c.index + 1) % c.files.length;
                playNextCarousel(layerId);
            }, ` + strconv.Itoa(carouselDelayMs) + `);
        }
    }

    function connectWS() {
        var ws = new WebSocket('ws://127.0.0.1:' + serverPort + '/ws');
        ws.onopen = function() {
            ws.send(JSON.stringify({action: "hello"}));
        };
        ws.onmessage = function(event) {
            try {
                var msg = JSON.parse(event.data);
                if (msg.action === 'sync') {
                    document.body.style.backgroundColor = msg.bgColor;
                    msg.layers.forEach(function(l) {
                        var container = document.getElementById(l.id);
                        if (container) {
                            container.style.cssText = l.style;
                        }
                        if (l.active) playLayer(l.id, l.files, l.volume);
                        else stopLayer(l.id);
                    });
                } else if (msg.action === "play" && msg.files) {
                    playLayer(msg.layer, msg.files, msg.volume);
                } else if (msg.action === "hide") {
                    stopLayer(msg.layer);
                } else if (msg.action === "volume" && msg.volume !== undefined) {
                    var container = document.getElementById(msg.layer);
                    if (container && container.firstElementChild) {
                        container.firstElementChild.volume = parseFloat(msg.volume) / 100.0;
                    }
                }
            } catch(e) {
                console.error('WebSocket payload parsing error:', e);
            }
        };
        ws.onclose = function() {
            setTimeout(connectWS, 2000);
        };
    }
    
    connectWS();
  </script>
</body>
</html>`

			htmlPath := "/opt/VLX_VisionBridge/var/overlay.html"
			if err := os.MkdirAll("/opt/VLX_VisionBridge/var", 0755); err == nil {
				if writeErr := os.WriteFile(htmlPath, []byte(htmlContent), 0644); writeErr != nil {
					log.Printf("Failed to generate overlay HTML artifact: %v", writeErr)
				}
			}

			log.Printf("Initializing Chromium browser natively on Xvfb Display :99")

			var fileURL string
			var cmdArgs []string

			cmdArgs = append(cmdArgs,
				"--kiosk",
				"--disable-infobars",
				"--disable-extensions",
				"--window-position=0,0",
				fmt.Sprintf("--window-size=%s,%s", resWidth, resHeight),
				"--autoplay-policy=no-user-gesture-required",
				"--disable-dev-shm-usage",
				"--no-sandbox",
				"--allow-file-access-from-files",
				"--disable-web-security",
				"--allow-running-insecure-content",
			)

			if cfg.Input.OverlayServerActive {
				fileURL = fmt.Sprintf("http://127.0.0.1:%d/overlay", overlayPort)
				cmdArgs = append(cmdArgs, fileURL)
			} else {
				fileURL = "file://" + htmlPath
				cmdArgs = append(cmdArgs, fileURL)
			}

			chromeBin, err := exec.LookPath("chromium")
			if err != nil {
				chromeBin, err = exec.LookPath("chromium-browser")
				if err != nil {
					log.Printf("Chromium browser executable not found in system path.")
					return
				}
			}

			cmd := exec.Command(chromeBin, cmdArgs...)
			cmd.Env = append(os.Environ(), "DISPLAY=:99", "PULSE_SERVER=unix:/tmp/pulse-visionbridge/native")

			err = cmd.Start()
			if err != nil {
				log.Printf("Failed to execute Chromium browser: %v", err)
			} else {
				pm.overlayCmds[99] = cmd
				go pm.monitorChromium(cmd)
			}
		}
	}

	for id, cmd := range pm.overlayCmds {
		if !activeOverlays[id] {
			if cmd != nil && cmd.Process != nil {
				log.Printf("Terminating overlay process for layer ID %d...", id)
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
			delete(pm.overlayCmds, id)
		}
	}
}

func (pm *ProcessManager) monitorChromium(cmd *exec.Cmd) {
	err := cmd.Wait()

	pm.mu.Lock()
	if !pm.isRunning || pm.ctx.Err() != nil {
		pm.mu.Unlock()
		return
	}

	if currentCmd, exists := pm.overlayCmds[99]; !exists || currentCmd != cmd {
		pm.mu.Unlock()
		return
	}
	delete(pm.overlayCmds, 99)
	pm.mu.Unlock()

	log.Printf("Chromium browser process exited unexpectedly: %v", err)
	if pm.db != nil {
		_ = db.LogStreamEvent(pm.db, "crash", fmt.Sprintf("Chromium browser crashed: %v", err))
	}
}

func (pm *ProcessManager) UpdateFilter(config *models.Config) {
	pm.mu.Lock()
	pm.config = config
	pm.mu.Unlock()
}

func (pm *ProcessManager) buildSyncMessage(cfg *models.Config) map[string]interface{} {
	type LayerState struct {
		ID     string   `json:"id"`
		Active bool     `json:"active"`
		Files  []string `json:"files"`
		Volume *int     `json:"volume"`
		Style  string   `json:"style"`
	}

	cs := cfg.Input.ChromiumSource
	bgColor := cfg.Input.BgColor
	if bgColor == "" {
		bgColor = "black"
	}

	return map[string]interface{}{
		"action":  "sync",
		"bgColor": bgColor,
		"layers": []LayerState{
			{ID: "z1", Active: cs.Z1Active, Files: pm.ResolvePath(cs.Z1Path), Volume: cs.Z1Volume, Style: buildLayerStyle(1, cs.Z1Width, cs.Z1Height, cs.Z1X, cs.Z1Y)},
			{ID: "z2", Active: cs.Z2Active, Files: pm.ResolvePath(cs.Z2Path), Volume: cs.Z2Volume, Style: buildLayerStyle(2, cs.Z2Width, cs.Z2Height, cs.Z2X, cs.Z2Y)},
			{ID: "z3", Active: cs.Z3Active, Files: pm.ResolvePath(cs.Z3Path), Volume: cs.Z3Volume, Style: buildLayerStyle(3, cs.Z3Width, cs.Z3Height, cs.Z3X, cs.Z3Y)},
			{ID: "z4", Active: cs.Z4Active, Files: pm.ResolvePath(cs.Z4Path), Volume: cs.Z4Volume, Style: buildLayerStyle(4, cs.Z4Width, cs.Z4Height, cs.Z4X, cs.Z4Y)},
			{ID: "z5", Active: cs.Z5Active, Files: pm.ResolvePath(cs.Z5Path), Volume: cs.Z5Volume, Style: buildLayerStyle(5, cs.Z5Width, cs.Z5Height, cs.Z5X, cs.Z5Y)},
			{ID: "z6", Active: cs.Z6Active, Files: pm.ResolvePath(cs.Z6Path), Volume: cs.Z6Volume, Style: buildLayerStyle(6, cs.Z6Width, cs.Z6Height, cs.Z6X, cs.Z6Y)},
			{ID: "z7", Active: cs.Z7Active, Files: pm.ResolvePath(cs.Z7Path), Volume: cs.Z7Volume, Style: buildLayerStyle(7, cs.Z7Width, cs.Z7Height, cs.Z7X, cs.Z7Y)},
			{ID: "z8", Active: cs.Z8Active, Files: pm.ResolvePath(cs.Z8Path), Volume: cs.Z8Volume, Style: buildLayerStyle(8, cs.Z8Width, cs.Z8Height, cs.Z8X, cs.Z8Y)},
			{ID: "z9", Active: cs.Z9Active, Files: pm.ResolvePath(cs.Z9Path), Volume: cs.Z9Volume, Style: buildLayerStyle(9, cs.Z9Width, cs.Z9Height, cs.Z9X, cs.Z9Y)},
		},
	}
}

// UpdateConfig evaluates runtime modifications and propagates them correctly.
func (pm *ProcessManager) UpdateConfig(config *models.Config) {
	pm.mu.Lock()
	
	// Detect if core overlay server settings have been modified
	restartServer := false
	if pm.config == nil ||
		pm.config.Input.OverlayServerActive != config.Input.OverlayServerActive ||
		pm.config.Input.OverlayServerPort != config.Input.OverlayServerPort ||
		pm.config.Input.MediaFolderPath != config.Input.MediaFolderPath {
		restartServer = true
	}

	pm.config = config

	if pm.cond != nil {
		pm.cond.Broadcast()
	}
	pm.mu.Unlock()

	if restartServer {
		log.Println("Core Overlay Server settings modified. Initiating warm restart of Control Server...")
		pm.startOverlayServer(config)
		pm.ReloadChromium()
	} else {
		syncMsg := pm.buildSyncMessage(config)
		pm.broadcastWSMessage(syncMsg)
		log.Println("Configuration successfully updated. Overlay layers synchronized dynamically via WebSocket.")
	}
}

type monitorAction int

const (
	monitorActionStop monitorAction = iota
	monitorActionContinue
	monitorActionSleepConstant
	monitorActionSleepExponential
)

func (pm *ProcessManager) monitor() {
	backoff := 1 * time.Second
	var lastBuildErr string

	for {
		pm.mu.Lock()
		isActive := true
		if pm.config != nil {
			isActive = pm.config.Output.Active
		}
		pm.mu.Unlock()

		if !isActive {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		action, finalModule, isMisconfig := pm.executeSingleRun(&lastBuildErr)

		if isMisconfig && finalModule != "" {
			log.Printf("Misconfiguration detected for module %s. Disabling the module.", finalModule)
			pm.disableModule(finalModule)
			continue
		}

		if action == monitorActionSleepExponential && finalModule != "" {
			pm.mu.Lock()
			tracker, exists := pm.retries[finalModule]
			if !exists {
				tracker = &RetryTracker{}
				pm.retries[finalModule] = tracker
			}
			if !tracker.LastCrash.IsZero() && time.Since(tracker.LastCrash) > 30*time.Second {
				tracker.ConsecutiveCrashes = 0
			}
			tracker.ConsecutiveCrashes++
			tracker.LastCrash = time.Now()
			crashes := tracker.ConsecutiveCrashes
			pm.mu.Unlock()

			if crashes <= 5 {
				log.Printf("Module %s crashed %d times (executing quick retry in 1s).", finalModule, crashes)
				time.Sleep(1 * time.Second)
				continue
			} else if crashes <= 7 {
				log.Printf("Module %s crashed %d times (executing delayed retry in 10s).", finalModule, crashes)
				time.Sleep(10 * time.Second)
				continue
			} else {
				log.Printf("Module %s crashed %d times. Maximum retry threshold exceeded. Disabling module.", finalModule, crashes)
				pm.disableModule(finalModule)
				continue
			}
		}

		switch action {
		case monitorActionStop:
			return
		case monitorActionContinue:
			continue
		case monitorActionSleepConstant:
			time.Sleep(backoff)
			continue
		case monitorActionSleepExponential:
			log.Printf("Restarting GStreamer pipeline in %v...", backoff)
			time.Sleep(backoff)

			backoff *= 2
			maxBackoff := 30 * time.Second
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (pm *ProcessManager) disableModule(module string) {
	if module == "" || pm.config == nil {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if module == "[chromium]" {
		pm.config.Input.ChromiumSource.Active = false
	} else if module == "[output]" {
		pm.config.Output.Active = false
	}
}

func identifyErrorModule(stderr string, cfg *models.Config) string {
	if cfg == nil || stderr == "" {
		return ""
	}

	for _, dest := range cfg.Output.Destinations {
		if dest == "" {
			continue
		}
		if strings.Contains(stderr, dest) {
			return "[output]"
		}
	}
	return ""
}

func (pm *ProcessManager) executeSingleRun(lastBuildErr *string) (monitorAction, string, bool) {
	pm.mu.Lock()
	ctx := pm.ctx
	cfg := pm.config
	isRunning := pm.isRunning
	pm.mu.Unlock()

	if !isRunning || ctx.Err() != nil {
		return monitorActionStop, "", false
	}

	pm.manageOverlays(cfg)

	args, err := BuildPipelineArgs(cfg)
	if err != nil {
		return monitorActionSleepConstant, "", false
	}

	if len(args) == 0 {
		pm.mu.Lock()
		if pm.isRunning && pm.ctx.Err() == nil {
			pm.cond.Wait()
		}
		pm.mu.Unlock()
		return monitorActionContinue, "", false
	}

	started, runErr, stderrStr := pm.runProcess(ctx, args)
	if !started {
		return monitorActionSleepConstant, "", false
	}

	if ctx.Err() != nil {
		return monitorActionStop, "", false
	}

	errMsg := "GStreamer pipeline exited unexpectedly."
	var finalModule string
	var isMisconfig bool

	if runErr != nil {
		if stderrStr != "" {
			lines := strings.Split(strings.TrimSpace(stderrStr), "\n")
			if len(lines) > 0 {
				isMisconfig = true
			}
			finalModule = identifyErrorModule(stderrStr, cfg)
		}
		errMsg = fmt.Sprintf("GStreamer crash detected: %v\n\n--- GSTREAMER ERROR DETAILS ---\n%s\n-------------------------------", runErr, strings.TrimSpace(stderrStr))
	}
	log.Println(errMsg)

	return monitorActionSleepExponential, finalModule, isMisconfig
}

func (pm *ProcessManager) runProcess(ctx context.Context, gstArgs []string) (bool, error, string) {
	gstCmd := exec.Command("gst-launch-1.0", gstArgs...)

	tb := &tailBuffer{}
	gstCmd.Stderr = tb

	gstCmd.Env = append(os.Environ(), "DISPLAY=:99", "PULSE_SERVER=unix:/tmp/pulse-visionbridge/native")

	pm.mu.Lock()
	pm.cmd = gstCmd
	pm.mu.Unlock()

	defer func() {
		pm.mu.Lock()
		pm.cmd = nil
		pm.mu.Unlock()
	}()

	log.Println("Starting GStreamer native X11 pipeline... (Transmitting stream to MediaMTX)")

	err := gstCmd.Start()
	if err != nil {
		return false, err, tb.String()
	}

	errWait, stderrStr := pm.waitForProcess(ctx, gstCmd)
	return true, errWait, stderrStr
}

func (pm *ProcessManager) waitForProcess(ctx context.Context, cmd *exec.Cmd) (error, string) {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var runErr error
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		runErr = <-done
	case runErr = <-done:
	}

	stderrStr := ""
	if tb, ok := cmd.Stderr.(*tailBuffer); ok {
		stderrStr = tb.String()
	}

	return runErr, stderrStr
}

// ReloadChromium provides a manual override to force a browser restart in case of critical rendering failures.
func (pm *ProcessManager) ReloadChromium() {
	pm.mu.Lock()
	if cmd, exists := pm.overlayCmds[99]; exists && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		delete(pm.overlayCmds, 99)
	}
	cfg := pm.config
	pm.mu.Unlock()

	if cfg != nil {
		pm.manageOverlays(cfg)
	}
}

// broadcastWSMessage securely transmits JSON payloads to all connected WebSocket clients.
func (pm *ProcessManager) broadcastWSMessage(msg interface{}) {
	pm.wsMutex.Lock()
	defer pm.wsMutex.Unlock()
	
	for client := range pm.wsClients {
		if err := client.WriteJSON(msg); err != nil {
			_ = client.Close()
			delete(pm.wsClients, client)
		}
	}
}
