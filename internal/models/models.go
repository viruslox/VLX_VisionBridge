// Package models defines the core data structures used for configuring and tracking the engine state.
package models

// OutputSettings defines the streaming output configuration for MediaMTX/FFmpeg external push.
type OutputSettings struct {
	Active          bool     `yaml:"active" json:"active"`
	Resolution      string   `yaml:"resolution"`
	FPS             int      `yaml:"fps"`
	VideoBitrate    string   `yaml:"video_bitrate"`
	AudioBitrate    string   `yaml:"audio_bitrate"`
	AudioSampleRate int      `yaml:"audio_sample_rate" json:"audio_sample_rate"`
	Destinations    []string `yaml:"destinations"`
}

// ChromiumSource defines the settings for up to 13 native DOM Z-layers (Z0-Z12) manipulated via WebSocket.
type ChromiumSource struct {
	Active   bool   `yaml:"active"`

	Z0Active bool   `yaml:"z0_active"`
	Z0Path   string `yaml:"z0_path"`
	Z0Volume *int   `yaml:"z0_volume"`
	Z0Width  *int   `yaml:"z0_width"`
	Z0Height *int   `yaml:"z0_height"`
	Z0X      *int   `yaml:"z0_x"`
	Z0Y      *int   `yaml:"z0_y"`

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

	Z9Active bool   `yaml:"z9_active"`
	Z9Path   string `yaml:"z9_path"`
	Z9Volume *int   `yaml:"z9_volume"`
	Z9Width  *int   `yaml:"z9_width"`
	Z9Height *int   `yaml:"z9_height"`
	Z9X      *int   `yaml:"z9_x"`
	Z9Y      *int   `yaml:"z9_y"`

	Z10Active bool   `yaml:"z10_active"`
	Z10Path   string `yaml:"z10_path"`
	Z10Volume *int   `yaml:"z10_volume"`
	Z10Width  *int   `yaml:"z10_width"`
	Z10Height *int   `yaml:"z10_height"`
	Z10X      *int   `yaml:"z10_x"`
	Z10Y      *int   `yaml:"z10_y"`

	Z11Active bool   `yaml:"z11_active"`
	Z11Path   string `yaml:"z11_path"`
	Z11Volume *int   `yaml:"z11_volume"`
	Z11Width  *int   `yaml:"z11_width"`
	Z11Height *int   `yaml:"z11_height"`
	Z11X      *int   `yaml:"z11_x"`
	Z11Y      *int   `yaml:"z11_y"`

	Z12Active bool   `yaml:"z12_active"`
	Z12Path   string `yaml:"z12_path"`
	Z12Volume *int   `yaml:"z12_volume"`
	Z12Width  *int   `yaml:"z12_width"`
	Z12Height *int   `yaml:"z12_height"`
	Z12X      *int   `yaml:"z12_x"`
	Z12Y      *int   `yaml:"z12_y"`
}

// InputSettings defines the main engine inputs, including base resolution and global background properties.
type InputSettings struct {
	BgColor             string         `yaml:"bg_color" json:"bg_color"`
	Resolution          string         `yaml:"resolution" json:"resolution"`
	Framerate           int            `yaml:"framerate" json:"framerate"`
	CarouselDelay       int            `yaml:"carousel_delay" json:"carousel_delay"`
	CarouselShuffle     bool           `yaml:"carousel_shuffle" json:"carousel_shuffle"`
	WebrtcPortMin       int            `yaml:"webrtc_port_min"`
	WebrtcPortMax       int            `yaml:"webrtc_port_max"`
	OverlayServerActive bool           `yaml:"overlay_server_active" json:"overlay_server_active"`
	OverlayServerPort   int            `yaml:"overlay_server_port" json:"overlay_server_port"`
	MediaFolderPath     string         `yaml:"media_folder_path" json:"media_folder_path"`
	ChromiumSource      ChromiumSource `yaml:"chromium_source"`
}

// InputResult represents the generated GStreamer argument slice and state assessment flags.
type InputResult struct {
	Args       []string
	InputCount int
	HasVideo   bool
	HasAudio   bool
}

// DatabaseConfig defines local SQLite schema configurations.
type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

// ConnectorSettings defines the IPC/ZMQ socket configuration parameters for local control routing.
type ConnectorSettings struct {
	IPCControlIn  bool   `yaml:"ipc_control_in" json:"ipc_control_in"`
	Group         string `yaml:"group" json:"group"`
	ControlSocket string `yaml:"control_socket" json:"control_socket"`
}

// Config is the root struct abstracting the config.yaml application state.
type Config struct {
	Database  DatabaseConfig    `yaml:"database"`
	Connector ConnectorSettings `yaml:"connector"`
	Output    OutputSettings    `yaml:"output"`
	Input     InputSettings     `yaml:"input"`
}
