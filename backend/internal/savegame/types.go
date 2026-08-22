package savegame

// ServerGameState is the nested state from QueryServerState.
type ServerGameState struct {
	ActiveSessionName string `json:"activeSessionName"`
	IsGameRunning     bool   `json:"isGameRunning"`
}

type queryServerStateData struct {
	ServerGameState ServerGameState `json:"serverGameState"`
}

type enumerateSessionsData struct {
	Sessions            []Session `json:"sessions"`
	CurrentSessionIndex int       `json:"currentSessionIndex"`
}

// Session groups save files for one playthrough.
type Session struct {
	SessionName string       `json:"sessionName"`
	SaveHeaders []SaveHeader `json:"saveHeaders"`
}

// SaveHeader describes one .sav file on the dedicated server.
type SaveHeader struct {
	SaveName     string `json:"saveName"`
	SessionName  string `json:"sessionName"`
	SaveDateTime string `json:"saveDateTime"`
	BuildVersion int    `json:"buildVersion"`
	IsModdedSave bool   `json:"isModdedSave"`
}

// LatestSaveInfo is metadata for the resolved latest autosave.
type LatestSaveInfo struct {
	ActiveSessionName string
	SaveName          string
	SaveDateTime      string
}

// DownloadResult is a ready-to-stream save file from the upstream API.
type DownloadResult struct {
	Filename string
	Size     int64
	Body     []byte
}
