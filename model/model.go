package model

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type Fan struct {
	PwmIndex   int    `json:"pwmIndex"`
	Label      string `json:"label,omitempty"`
	Active     bool   `json:"active"`
	RPM        int    `json:"rpm"`
	PWMRaw     int    `json:"pwmRaw"`
	PWMPercent int    `json:"pwmPercent"`
	PWMMin     int    `json:"pwmMin"`
	PWMMax     int    `json:"pwmMax"`
	Mode       string `json:"mode"`
	Alarm      bool   `json:"alarm,omitempty"`
}

type Chip struct {
	HwmonIndex int    `json:"hwmonIndex"`
	HwmonName  string `json:"hwmonName"`
	Fans       []Fan  `json:"fans"`
}

type FansResponse struct {
	Chips []Chip `json:"chips"`
}

type SetFanSpeedRequest struct {
	HwmonIndex int    `json:"hwmonIndex"`
	PwmIndex   int    `json:"pwmIndex"`
	Mode       string `json:"mode,omitempty"`
	PWMRaw     *int   `json:"pwmRaw,omitempty"`
	PWMPercent *int   `json:"pwmPercent,omitempty"`
}

type SetFanSpeedResponse struct {
	HwmonIndex int    `json:"hwmonIndex"`
	HwmonName  string `json:"hwmonName"`
	Fan        Fan    `json:"fan"`
}
