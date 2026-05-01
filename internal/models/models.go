package models

type OutputSettings struct {
	Resolution   string   `yaml:"resolution"`
	FPS          int      `yaml:"fps"`
	VideoBitrate string   `yaml:"video_bitrate"`
	AudioBitrate string   `yaml:"audio_bitrate"`
	Destinations []string `yaml:"destinations"`
}

type Layer struct {
	ID        int    `yaml:"id"`
	Active    bool   `yaml:"active"`
	InputType string `yaml:"input_type"` // e.g., folder, loop, srt
	InputPath string `yaml:"input_path"`
	Media     string `yaml:"media"`      // Video+Audio, Video Only, Audio Only
	Scale     string `yaml:"scale"`
	Crop      string `yaml:"crop"`
	Position  string `yaml:"position"`
}

type Config struct {
	Output OutputSettings `yaml:"output"`
	Layers []Layer        `yaml:"layers"`
}
