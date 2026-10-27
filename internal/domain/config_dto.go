package domain

// MeResponse is the authenticated API user profile.
type MeResponse struct {
	Username  string                    `json:"username"`
	Databases map[string][]AccessLevel  `json:"databases"`
}

// ServerConfigResponse is non-secret server settings for the client.
type ServerConfigResponse struct {
	DefaultWorkstation string            `json:"default_workstation"`
	Workstations       []WorkstationInfo `json:"workstations"`
	SearchModes        []string          `json:"search_modes"`
	Irbis              IrbisServerInfo   `json:"irbis"`
}

// IrbisServerInfo is live (or cached) IRBIS metadata.
type IrbisServerInfo struct {
	Version          string         `json:"version,omitempty"`
	Organization     string         `json:"organization,omitempty"`
	MaxClients       int            `json:"max_clients,omitempty"`
	ConnectedClients int            `json:"connected_clients,omitempty"`
	Databases        []DatabaseInfo `json:"databases,omitempty"`
}

// DatabaseInfo is one database from the server menu.
type DatabaseInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ReadOnly    bool   `json:"read_only"`
	MaxMFN      int    `json:"max_mfn,omitempty"`
}

// AllSearchModes lists modes accepted by BuildExpression.
func AllSearchModes() []string {
	return []string{
		string(ModeExpression),
		string(ModeAuthor),
		string(ModeTitle),
		string(ModeKeyword),
		string(ModeInventory),
		string(ModeIndex),
		string(ModeYear),
		string(ModePublisher),
		string(ModeSubject),
		string(ModeLanguage),
		string(ModeAll),
	}
}
