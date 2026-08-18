package connection

// Details holds game join connection settings (§8.2).
type Details struct {
	GameHost        string `json:"gameHost"`
	GamePort        int    `json:"gamePort"`
	GamePassword    string `json:"gamePassword,omitempty"`
	Notes           string `json:"notes,omitempty"`
	SMMProfileName  string `json:"smmProfileName,omitempty"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
	UpdatedByUserID *int64 `json:"updatedByUserId,omitempty"`
}

// UpdateInput is the mutable subset of connection details.
type UpdateInput struct {
	GameHost       *string `json:"gameHost"`
	GamePort       *int    `json:"gamePort"`
	GamePassword   *string `json:"gamePassword"`
	Notes          *string `json:"notes"`
	SMMProfileName *string `json:"smmProfileName"`
	ClearPassword  bool    `json:"clearPassword"`
}
