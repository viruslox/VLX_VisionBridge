package engine

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/pion/webrtc/v3"
)

var (
	webrtcPeerConnection *webrtc.PeerConnection
	webrtcMutex          sync.Mutex
	WebRTCVideoTrack     *webrtc.TrackRemote
	WebRTCAudioTrack     *webrtc.TrackRemote
)

func startWebRTCServer() {
	http.HandleFunc("/webrtc/offer", handleWebRTCOffer)
	log.Println("Starting WebRTC signaling server on :50000")
	if err := http.ListenAndServe(":50000", nil); err != nil {
		log.Printf("WebRTC signaling server failed: %v", err)
	}
}

func handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(body),
	}

	webrtcMutex.Lock()
	defer webrtcMutex.Unlock()

	if webrtcPeerConnection != nil {
		webrtcPeerConnection.Close()
	}

	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Got WebRTC Track: %s (%s)", track.Kind().String(), track.Codec().MimeType)

		port := 50002
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			port = 50003
		}

		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			log.Printf("Failed to resolve UDP addr: %v", err)
			return
		}
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			log.Printf("Failed to dial UDP: %v", err)
			return
		}
		defer conn.Close()

		b := make([]byte, 1500)
		for {
			n, _, readErr := track.Read(b)
			if readErr != nil {
				return
			}
			if _, writeErr := conn.Write(b[:n]); writeErr != nil {
				return
			}
		}
	})

	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("WebRTC Peer Connection State has changed: %s", s.String())
	})

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	webrtcPeerConnection = peerConnection

	w.Header().Set("Content-Type", "application/sdp")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(answer.SDP))
}
