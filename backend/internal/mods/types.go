package mods

// Mod is a server mod row for API responses (§8.5).
type Mod struct {
	Name               string `json:"name"`
	SMRName            string `json:"smrName"`
	Version            string `json:"version"`
	Description        string `json:"description,omitempty"`
	DocsURL            string `json:"docsUrl,omitempty"`
	SupportURL         string `json:"supportUrl,omitempty"`
	CreatedBy          string `json:"createdBy,omitempty"`
	RemoteVersionRange string `json:"remoteVersionRange,omitempty"`
	RequiredOnRemote   bool   `json:"requiredOnRemote"`
}

// ListResponse is the GET /api/mods envelope.
type ListResponse struct {
	GameBuild    string `json:"gameBuild"`
	SMLVersion   string `json:"smlVersion"`
	Mods         []Mod  `json:"mods"`
	CachedAt     string `json:"cachedAt"`
	FRMReachable bool   `json:"frmReachable"`
}
