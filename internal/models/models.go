package models

type OutputSettings struct {
	Active       bool     `yaml:"active" json:"active"`
	Resolution   string   `yaml:"resolution"`
	FPS          int      `yaml:"fps"`
	VideoBitrate string   `yaml:"video_bitrate"`
	AudioBitrate string   `yaml:"audio_bitrate"`
	Destinations []string `yaml:"destinations"`
}

type FFmpegSource struct {
	Active bool    `yaml:"active"`
	Layers []Layer `yaml:"layers"`
}

type ChromiumSource struct {
	Active   bool   `yaml:"active"`
	Z1Active bool   `yaml:"z1_active"`
	Z1Path   string `yaml:"z1_path"`
	Z1Volume *int   `yaml:"z1_volume"`
	Z1Width  *int   `yaml:"z1_width"`
	Z1Height *int   `yaml:"z1_height"`
	Z1X      *int   `yaml:"z1_x"`
	Z1Y      *int   `yaml:"z1_y"`

	Z2Active bool   `yaml:"z2_active"`
	Z2Path   string `yaml:"z2_path"`
	Z2Volume *int   `yaml:"z2_volume"`
	Z2Width  *int   `yaml:"z2_width"`
	Z2Height *int   `yaml:"z2_height"`
	Z2X      *int   `yaml:"z2_x"`
	Z2Y      *int   `yaml:"z2_y"`

	Z3Active bool   `yaml:"z3_active"`
	Z3Path   string `yaml:"z3_path"`
	Z3Volume *int   `yaml:"z3_volume"`
	Z3Width  *int   `yaml:"z3_width"`
	Z3Height *int   `yaml:"z3_height"`
	Z3X      *int   `yaml:"z3_x"`
	Z3Y      *int   `yaml:"z3_y"`

	Z4Active bool   `yaml:"z4_active"`
	Z4Path   string `yaml:"z4_path"`
	Z4Volume *int   `yaml:"z4_volume"`
	Z4Width  *int   `yaml:"z4_width"`
	Z4Height *int   `yaml:"z4_height"`
	Z4X      *int   `yaml:"z4_x"`
	Z4Y      *int   `yaml:"z4_y"`

	Z5Active bool   `yaml:"z5_active"`
	Z5Path   string `yaml:"z5_path"`
	Z5Volume *int   `yaml:"z5_volume"`
	Z5Width  *int   `yaml:"z5_width"`
	Z5Height *int   `yaml:"z5_height"`
	Z5X      *int   `yaml:"z5_x"`
	Z5Y      *int   `yaml:"z5_y"`

	Z6Active bool   `yaml:"z6_active"`
	Z6Path   string `yaml:"z6_path"`
	Z6Volume *int   `yaml:"z6_volume"`
	Z6Width  *int   `yaml:"z6_width"`
	Z6Height *int   `yaml:"z6_height"`
	Z6X      *int   `yaml:"z6_x"`
	Z6Y      *int   `yaml:"z6_y"`

	Z7Active bool   `yaml:"z7_active"`
	Z7Path   string `yaml:"z7_path"`
	Z7Volume *int   `yaml:"z7_volume"`
	Z7Width  *int   `yaml:"z7_width"`
	Z7Height *int   `yaml:"z7_height"`
	Z7X      *int   `yaml:"z7_x"`
	Z7Y      *int   `yaml:"z7_y"`

	Z8Active bool   `yaml:"z8_active"`
	Z8Path   string `yaml:"z8_path"`
	Z8Volume *int   `yaml:"z8_volume"`
	Z8Width  *int   `yaml:"z8_width"`
	Z8Height *int   `yaml:"z8_height"`
	Z8X      *int   `yaml:"z8_x"`
	Z8Y      *int   `yaml:"z8_y"`
}

type InputSettings struct {
	Resolution     string         `yaml:"resolution"`
	FFmpegSource   FFmpegSource   `yaml:"ffmpeg_source"`
	ChromiumSource ChromiumSource `yaml:"chromium_source"`
}

type FolderOptions struct {
	IsFolder bool `yaml:"is_folder" json:"is_folder"`
	Shuffle  bool `yaml:"shuffle" json:"shuffle"`
	Loop     bool `yaml:"loop" json:"loop"`
	DelaySec int  `yaml:"delay_sec" json:"delay_sec"`
}

type Layer struct {
	ID            int           `yaml:"id"`
	Active        bool          `yaml:"active"`
	InputType     string        `yaml:"input_type"` // e.g., folder, loop, srt
	InputPath     string        `yaml:"input_path"`
	Media         string        `yaml:"media"` // Video+Audio, Video Only, Audio Only
	Size          int           `yaml:"size"`
	X             int           `yaml:"x"`
	Y             int           `yaml:"y"`
	Volume        *int          `yaml:"volume"`
	FolderOptions FolderOptions `yaml:"folder_options"`
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

type ConnectorSettings struct {
	IPCControlIn  bool   `yaml:"ipc_control_in" json:"ipc_control_in"`
	Group         string `yaml:"group" json:"group"`
	ControlSocket string `yaml:"control_socket" json:"control_socket"`
}

type Config struct {
	Database  DatabaseConfig    `yaml:"database"`
	Connector ConnectorSettings `yaml:"connector"`
	Output    OutputSettings    `yaml:"output"`
	Input     InputSettings     `yaml:"input"`
}
