package configs

import _ "embed"

//go:embed visionbridge.settings.template
var SettingsTemplate []byte

//go:embed frontend.settings.template
var FrontendSettingsTemplate []byte
