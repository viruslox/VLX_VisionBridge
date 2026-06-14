package mixer

import (
	"testing"

	"github.com/user/VLX_VisionBridge/internal/models"
)

func TestBuildFilterComplex(t *testing.T) {
	vol100 := 100
	vol50 := 50

	tests := []struct {
		name              string
		cfg               *models.Config
		expectedArgs      []string
		expectedFilterStr string
		expectedVideoPad  string
		expectedAudioPad  string
	}{
		{
			name: "No Inputs",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
				},
			},
			expectedArgs:      nil,
			expectedFilterStr: "color=s=1920x1080:c=black [base];\nanullsrc=r=44100:cl=stereo [a_out];\n[base] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a_out]",
		},
		{
			name: "FFmpeg Source Video and Audio",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
					FFmpegSource: models.FFmpegSource{
						Active: true,
						Layers: []models.Layer{
							{
								ID:        1,
								Active:    true,
								InputType: "local",
								InputPath: "test.mp4",
								Media:     "Video+Audio",
								Size:      1920,
								X:         0,
								Y:         0,
								Volume:    &vol100,
							},
						},
					},
				},
			},
			expectedArgs:      []string{"-i", "test.mp4"},
			expectedFilterStr: "color=s=1920x1080:c=black [base];\n[0:v] scale=1920:-1 [v0_scaled];\n[base][v0_scaled] overlay@layer1=x=0:y=0 [out0];\n[0:a] aresample=48000,aformat=sample_rates=48000:channel_layouts=stereo, volume@layer1=1.00 [a0];\n[out0] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a0]",
		},
		{
			name: "FFmpeg Source Video Only",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
					FFmpegSource: models.FFmpegSource{
						Active: true,
						Layers: []models.Layer{
							{
								ID:        1,
								Active:    true,
								InputType: "local",
								InputPath: "test.mp4",
								Media:     "Video",
								Size:      1920,
								X:         0,
								Y:         0,
							},
						},
					},
				},
			},
			expectedArgs:      []string{"-i", "test.mp4"},
			expectedFilterStr: "color=s=1920x1080:c=black [base];\n[0:v] scale=1920:-1 [v0_scaled];\n[base][v0_scaled] overlay@layer1=x=0:y=0 [out0];\nanullsrc=r=44100:cl=stereo [a_out];\n[out0] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a_out]",
		},
		{
			name: "FFmpeg Source Audio Only",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
					FFmpegSource: models.FFmpegSource{
						Active: true,
						Layers: []models.Layer{
							{
								ID:        1,
								Active:    true,
								InputType: "local",
								InputPath: "test.mp3",
								Media:     "Audio",
								Volume:    &vol50,
							},
						},
					},
				},
			},
			expectedArgs:      []string{"-i", "test.mp3"},
			expectedFilterStr: "color=s=1920x1080:c=black [base];\n[0:a] aresample=48000,aformat=sample_rates=48000:channel_layouts=stereo, volume@layer1=0.50 [a0];\n[base] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a0]",
		},
		{
			name: "Multiple FFmpeg Sources",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
					FFmpegSource: models.FFmpegSource{
						Active: true,
						Layers: []models.Layer{
							{
								ID:        1,
								Active:    true,
								InputType: "local",
								InputPath: "test1.mp4",
								Media:     "Video+Audio",
								Size:      1280,
								X:         0,
								Y:         0,
								Volume:    &vol100,
							},
							{
								ID:        2,
								Active:    true,
								InputType: "local",
								InputPath: "test2.mp4",
								Media:     "Video+Audio",
								Size:      640,
								X:         1280,
								Y:         0,
								Volume:    &vol50,
							},
						},
					},
				},
			},
			expectedArgs:      []string{"-i", "test1.mp4", "-i", "test2.mp4"},
			expectedFilterStr: "color=s=1920x1080:c=black [base];\n[0:v] scale=1280:-1 [v0_scaled];\n[base][v0_scaled] overlay@layer1=x=0:y=0 [out0];\n[0:a] aresample=48000,aformat=sample_rates=48000:channel_layouts=stereo, volume@layer1=1.00 [a0];\n[1:v] scale=640:-1 [v1_scaled];\n[out0][v1_scaled] overlay@layer2=x=1280:y=0 [out1];\n[1:a] aresample=48000,aformat=sample_rates=48000:channel_layouts=stereo, volume@layer2=0.50 [a1];\n[a0][a1] amix=inputs=2:duration=longest [a_out];\n[out1] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a_out]",
		},
		{
			name: "Chromium Source Active",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
					ChromiumSource: models.ChromiumSource{
						Active:    true,
						Z1BgColor: "#00FF00",
					},
				},
			},
			expectedArgs:      []string{"-f", "x11grab", "-video_size", "1920x1080", "-draw_mouse", "0", "-i", ":99"},
			expectedFilterStr: "color=s=1920x1080:c=black [base];\n[0:v] colorkey=0x00FF00:0.1:0.1 [chroma_chromium];\n[base][chroma_chromium] overlay=x=0:y=0 [out_chromium];\nanullsrc=r=44100:cl=stereo [a_out];\n[out_chromium] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a_out]",
		},
		{
			name: "Layer Media Empty Falls Back to Video+Audio",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
					FFmpegSource: models.FFmpegSource{
						Active: true,
						Layers: []models.Layer{
							{
								ID:        1,
								Active:    true,
								InputType: "local",
								InputPath: "test.mp4",
								Media:     "", // Empty should default to Video+Audio
								Size:      1920,
								X:         0,
								Y:         0,
								Volume:    &vol100,
							},
						},
					},
				},
			},
			expectedArgs:      []string{"-i", "test.mp4"},
			expectedFilterStr: "color=s=1920x1080:c=black [base];\n[0:v] scale=1920:-1 [v0_scaled];\n[base][v0_scaled] overlay@layer1=x=0:y=0 [out0];\n[0:a] aresample=48000,aformat=sample_rates=48000:channel_layouts=stereo, volume@layer1=1.00 [a0];\n[out0] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a0]",
		},
		{
			name: "Missing Volume Falls Back to 1.0",
			cfg: &models.Config{
				Input: models.InputSettings{
					Resolution: "1920x1080",
					FFmpegSource: models.FFmpegSource{
						Active: true,
						Layers: []models.Layer{
							{
								ID:        1,
								Active:    true,
								InputType: "local",
								InputPath: "test.mp4",
								Media:     "Video+Audio",
								Size:      1920,
								X:         0,
								Y:         0,
								Volume:    nil, // nil should default to 1.00
							},
						},
					},
				},
			},
			expectedArgs:      []string{"-i", "test.mp4"},
			expectedFilterStr: "color=s=1920x1080:c=black [base];\n[0:v] scale=1920:-1 [v0_scaled];\n[base][v0_scaled] overlay@layer1=x=0:y=0 [out0];\n[0:a] aresample=48000,aformat=sample_rates=48000:channel_layouts=stereo, volume@layer1=1.00 [a0];\n[out0] zmq=b=tcp\\\\://127.0.0.1\\\\:5555 [v_out];\n",
			expectedVideoPad:  "[v_out]",
			expectedAudioPad:  "[a0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, filterStr, videoPad, audioPad := BuildFilterComplex(tt.cfg)

			if len(args) != len(tt.expectedArgs) {
				t.Errorf("expected %d args, got %d", len(tt.expectedArgs), len(args))
			} else {
				for i, arg := range args {
					if arg != tt.expectedArgs[i] {
						t.Errorf("expected arg %d to be %q, got %q", i, tt.expectedArgs[i], arg)
					}
				}
			}

			if filterStr != tt.expectedFilterStr {
				t.Errorf("expected filter complex string to be:\n%q\nGot:\n%q", tt.expectedFilterStr, filterStr)
			}

			if videoPad != tt.expectedVideoPad {
				t.Errorf("expected video pad to be %q, got %q", tt.expectedVideoPad, videoPad)
			}

			if audioPad != tt.expectedAudioPad {
				t.Errorf("expected audio pad to be %q, got %q", tt.expectedAudioPad, audioPad)
			}
		})
	}
}
