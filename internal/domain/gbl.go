package domain

// GblStatement is one global-correction operator.
type GblStatement struct {
	Command    string `json:"command"`
	Parameter1 string `json:"parameter1"`
	Parameter2 string `json:"parameter2"`
	Format1    string `json:"format1"`
	Format2    string `json:"format2"`
}

// GblRequest is a global correction job submitted via API.
type GblRequest struct {
	Actualize        bool           `json:"actualize"`
	Autoin           bool           `json:"autoin"`
	FormalControl    bool           `json:"formal_control"`
	Filename         string         `json:"filename"`
	SearchExpression string         `json:"search_expression"`
	Mode             SearchMode     `json:"mode"`
	Value            string         `json:"value"`
	FirstRecord      int            `json:"first_record"`
	NumberOfRecords  int            `json:"number_of_records"`
	MinMFN           int            `json:"min_mfn"`
	MaxMFN           int            `json:"max_mfn"`
	MfnList          []int          `json:"mfn_list"`
	Statements       []GblStatement `json:"statements"`
}

// GblResult is the server response lines from a GBL run.
type GblResult struct {
	Lines []string `json:"lines"`
}
