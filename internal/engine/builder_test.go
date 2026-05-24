package engine

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestBuildFFmpegArgs(t *testing.T) {
	tmpDir := t.TempDir()
	imagesDir := tmpDir + "/images"
	err := os.MkdirAll(imagesDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create mock images dir: %v", err)
	}
	os.WriteFile(imagesDir+"/test.png", []byte("mock image"), 0644)

	cfg := &models.Config{
		Input: models.InputSettings{
			Resolution: "1920x1080",
			FFmpegSource: models.FFmpegSource{
				Active: true,
				Layers: []models.Layer{
					{
						ID:        0,
						Active:    true,
						InputType: "local",
						InputPath: imagesDir,
						Size:      1920,
						X:         96,
						Y:         54,
						Media:     "Video+Audio",
					},
					{
						ID:        1,
						Active:    true,
						InputType: "local",
						InputPath: "video.mp4",
						Size:      960,
						X:         480,
						Y:         270,
						Media:     "Video+Audio",
					},
					{
						ID:        2,
						Active:    false, // Should be ignored
						InputType: "srt",
						InputPath: "srt://example.com:1234",
						X:         0,
						Y:         0,
					},
					{
						ID:        3,
						Active:    true,
						InputType: "srt",
						InputPath: "srt://example.com:5678",
						Size:      1280,
						X:         10,
						Y:         20,
						Media:     "Video+Audio",
					},
				},
			},
		},
		Output: models.OutputSettings{
			Resolution:   "1920x1080",
			FPS:          60,
			VideoBitrate: "6000k",
			AudioBitrate: "160k",
			Destinations: []string{
				"rtmp://live.twitch.tv/app/live_xyz",
				"rtmp://a.rtmp.youtube.com/live2/xyz",
			},
		},
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Failed to build args: %v", err)
	}

	argsStr := strings.Join(args, " ")

	// 1. Verify inputs
	if !strings.Contains(argsStr, "-i "+imagesDir) {
		t.Errorf("Missing fallback input for image dir: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-i video.mp4") {
		t.Errorf("Missing fallback input: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-i srt://example.com:5678") {
		t.Errorf("Missing active srt input: %s", argsStr)
	}
	if strings.Contains(argsStr, "srt://example.com:1234") {
		t.Errorf("Included inactive srt input: %s", argsStr)
	}

	// 2. Verify filter complex
	var filterComplexStr string
	for i, arg := range args {
		if arg == "-filter_complex" && i+1 < len(args) {
			filterComplexStr = args[i+1]
			break
		}
	}
	if filterComplexStr == "" {
		t.Fatalf("Missing -filter_complex flag")
	}

	// Layer 0 (x=96, y=54)
	if !strings.Contains(filterComplexStr, "overlay@layer0=x=96:y=54 [out0]") {
		t.Errorf("Layer 0 missing x=96:y=54 overlay: %s", filterComplexStr)
	}

	// Layer 1 (x=480, y=270)
	if !strings.Contains(filterComplexStr, "overlay@layer1=x=480:y=270 [out1]") {
		t.Errorf("Layer 1 missing x=480:y=270 overlay: %s", filterComplexStr)
	}

	// Layer 3 (custom pos 10:20)
	// Notice index is 3 in layers array, so input is [3:v] and out is [out3]
	if !strings.Contains(filterComplexStr, "overlay@layer3=x=10:y=20 [out3]") {
		t.Errorf("Layer 3 missing custom pos overlay: %s", filterComplexStr)
	}

	// Verify scaling logic
	// Layer 1 scale 960 (which represents previous 50% of 1920)
	if !strings.Contains(filterComplexStr, "scale=960:-1 [v1_scaled]") {
		t.Errorf("Layer 1 missing 960 scale: %s", filterComplexStr)
	}

	// Layer 3 absolute scale size 1280
	if !strings.Contains(filterComplexStr, "scale=1280:-1 [v3_scaled]") {
		t.Errorf("Layer 3 missing 1280 scale: %s", filterComplexStr)
	}

	// 3. Verify final map
	if !strings.Contains(argsStr, "-map [v_out]") {
		t.Errorf("Missing final map to last active layer video: %s", argsStr)
	}

	// 4. Verify global settings
	if !strings.Contains(argsStr, "-s 1920x1080") {
		t.Errorf("Missing Resolution setting: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-r 60") {
		t.Errorf("Missing FPS setting: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-c:v libx264 -pix_fmt yuv420p -b:v 6000k -maxrate 6000k -bufsize 6000k") {
		t.Errorf("Missing VideoBitrate setting: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-c:a aac -b:a 160k") {
		t.Errorf("Missing AudioBitrate setting: %s", argsStr)
	}

	// 5. Verify tee muxer
	expectedTee := "-f tee -use_fifo 1 -fifo_options drop_pkts_on_overflow=1:attempt_recovery=1:recovery_wait_time=1 [f=flv]rtmp://live.twitch.tv/app/live_xyz|[f=flv]rtmp://a.rtmp.youtube.com/live2/xyz"
	if !strings.Contains(argsStr, expectedTee) {
		t.Errorf("Missing or incorrect tee muxer setting: expected %s in %s", expectedTee, argsStr)
	}
}

func TestBuildFFmpegArgs_TeeMuxerInjection(t *testing.T) {
	cfg := &models.Config{
			Input: models.InputSettings{
				Resolution: "1920x1080",
				FFmpegSource: models.FFmpegSource{
					Active: true,
					Layers: []models.Layer{
						{
							ID:        0,
							Active:    true,
							InputType: "loop",
							InputPath: "video.mp4",
						},
					},
				},
			},
		Output: models.OutputSettings{
			Resolution: "1920x1080",
			FPS:        60,
			Destinations: []string{
				"rtmp://localhost/app/stream|[f=mp4]/tmp/pwned.mp4",
				"rtmp://localhost/app/stream2\\[f=flv]inject",
			},
		},
	}

	_, err := BuildFFmpegArgs(cfg)
	if err == nil {
		t.Fatalf("Expected error when building args with injected tee muxer strings, but got nil")
	}

	expectedErrMsg := "invalid or unsafe output destination"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error to contain %q, but got: %v", expectedErrMsg, err)
	}
}

func TestBuildFFmpegArgs_ValidDestinations(t *testing.T) {
	cfg := &models.Config{
			Input: models.InputSettings{
				Resolution: "1920x1080",
				FFmpegSource: models.FFmpegSource{
					Active: true,
					Layers: []models.Layer{
						{
							ID:        0,
							Active:    true,
							InputType: "loop",
							InputPath: "video.mp4",
						},
					},
				},
			},
		Output: models.OutputSettings{
			Resolution: "1920x1080",
			FPS:        60,
			Destinations: []string{
				"rtmp://live.twitch.tv/app/live_xyz",
				"srt://example.com:1234",
				"rtmps://live-api-s.facebook.com:443/rtmp/",
			},
		},
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Expected no error for valid destinations, got: %v", err)
	}

	argsStr := strings.Join(args, " ")
	expectedDest1 := "[f=flv]rtmp://live.twitch.tv/app/live_xyz"
	expectedDest2 := "[f=mpegts]srt://example.com:1234"
	expectedDest3 := "[f=flv]rtmps://live-api-s.facebook.com:443/rtmp/"
	expectedTeeMap := expectedDest1 + "|" + expectedDest2 + "|" + expectedDest3
	expectedTeeArg := "-f tee -use_fifo 1 -fifo_options drop_pkts_on_overflow=1:attempt_recovery=1:recovery_wait_time=1 " + expectedTeeMap

	if !strings.Contains(argsStr, expectedTeeArg) {
		t.Errorf("Tee muxer argument incorrect for valid destinations.\nExpected: %s\nGot args: %s", expectedTeeArg, argsStr)
	}
}

func TestBuildFFmpegArgs_10SRT(t *testing.T) {
	cfg := &models.Config{
		Input: models.InputSettings{
			Resolution: "1920x1080",
			FFmpegSource: models.FFmpegSource{
				Active: true,
				Layers: make([]models.Layer, 10),
			},
		},
		Output: models.OutputSettings{
			Resolution: "1920x1080",
			FPS:        60,
		},
	}

	for i := 0; i < 10; i++ {
		cfg.Input.FFmpegSource.Layers[i] = models.Layer{
			ID:        i,
			Active:    true,
			InputType: "srt",
			InputPath: "srt://example.com:" + strconv.Itoa(10000+i),
			Size:      192,
			X:         0,
			Y:         0,
		}
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Failed to build args: %v", err)
	}

	argsStr := strings.Join(args, " ")

	// Verify all 10 SRT inputs are present with low latency flags
	for i := 0; i < 10; i++ {
		inputStr := "-fflags nobuffer -flags low_delay -i srt://example.com:" + strconv.Itoa(10000+i)
		if !strings.Contains(argsStr, inputStr) {
			t.Errorf("Missing or incorrect low latency SRT input %d: expected %s", i, inputStr)
		}
	}

	// Verify filter complex has 10 inputs mapped
	var filterComplexStr string
	for i, arg := range args {
		if arg == "-filter_complex" && i+1 < len(args) {
			filterComplexStr = args[i+1]
			break
		}
	}
	if filterComplexStr == "" {
		t.Fatalf("Missing -filter_complex flag")
	}

	for i := 0; i < 10; i++ {
		if !strings.Contains(filterComplexStr, "["+strconv.Itoa(i)+":v]") {
			t.Errorf("Missing input mapping for layer %d", i)
		}
	}
}

func TestBuildFFmpegArgs_InactiveSources(t *testing.T) {
	cfg := &models.Config{
		Input: models.InputSettings{
			Resolution: "1920x1080",
			FFmpegSource: models.FFmpegSource{
				Active: true,
				Layers: []models.Layer{
					{
						ID:        0,
						Active:    true,
						InputType: "loop",
						InputPath: "active_video.mp4",
						Size:      1920,
						X:         0,
						Y:         0,
					},
					{
						ID:        1,
						Active:    false,
						InputType: "srt",
						InputPath: "srt://example.com:9999",
						X:         0,
						Y:         0,
					},
					{
						ID:        2,
						Active:    false,
						InputType: "local",
						InputPath: "/inactive_images/",
						X:         0,
						Y:         0,
					},
				},
			},
		},
		Output: models.OutputSettings{
			Resolution: "1920x1080",
			FPS:        60,
		},
	}

	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		t.Fatalf("Failed to build args: %v", err)
	}

	argsStr := strings.Join(args, " ")

	// Ensure active input is included
	if !strings.Contains(argsStr, "active_video.mp4") {
		t.Errorf("Active input missing from args")
	}

	// Ensure inactive inputs are NOT included
	if strings.Contains(argsStr, "srt://example.com:9999") {
		t.Errorf("Inactive SRT input found in args! It should consume 0 resources")
	}
	if strings.Contains(argsStr, "/inactive_images/") {
		t.Errorf("Inactive folder input found in args! It should consume 0 resources")
	}

	// Ensure there is only one input mapped [0:v], the others should be ignored
	var filterComplexStr string
	for i, arg := range args {
		if arg == "-filter_complex" && i+1 < len(args) {
			filterComplexStr = args[i+1]
			break
		}
	}

	if !strings.Contains(filterComplexStr, "[0:v]") {
		t.Errorf("Missing input mapping for active layer 0")
	}

	if strings.Contains(filterComplexStr, "[1:v]") {
		t.Errorf("Found input mapping for inactive layer 1, should be omitted")
	}

	if strings.Contains(filterComplexStr, "[2:v]") {
		t.Errorf("Found input mapping for inactive layer 2, should be omitted")
	}
}
