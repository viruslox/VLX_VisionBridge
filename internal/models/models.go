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
	Media     string `yaml:"media"` // Video+Audio, Video Only, Audio Only
	Size      int    `yaml:"size"`
	X         int    `yaml:"x"`
	Y         int    `yaml:"y"`
	Volume    *int   `yaml:"volume"`
}

type InputResult struct {
	Args       []string
	InputCount int
	HasVideo   bool
	HasAudio   bool
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Output   OutputSettings `yaml:"output"`
	Layers   []Layer        `yaml:"layers"`
}
