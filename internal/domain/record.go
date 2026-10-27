package domain

// SubField is one MARC subfield.
type SubField struct {
	Code  string `json:"code"`
	Value string `json:"value"`
}

// Field is one MARC field with optional subfields.
type Field struct {
	Tag       int        `json:"tag"`
	Value     string     `json:"value,omitempty"`
	Subfields []SubField `json:"subfields,omitempty"`
}

// Record is the JSON representation of an IRBIS MarcRecord.
type Record struct {
	Database  string  `json:"database"`
	MFN       int     `json:"mfn"`
	Status    int     `json:"status"`
	Version   int     `json:"version"`
	Fields    []Field `json:"fields"`
	Formatted string  `json:"formatted,omitempty"`
}

// RecordWrite is create/update payload.
type RecordWrite struct {
	MFN     int     `json:"mfn"`
	Status  int     `json:"status"`
	Version int     `json:"version"`
	Fields  []Field `json:"fields"`
}

// SearchMode builds IRBIS expressions from friendly modes.
type SearchMode string

const (
	ModeExpression SearchMode = "expression"
	ModeAuthor     SearchMode = "author"
	ModeTitle      SearchMode = "title"
	ModeKeyword    SearchMode = "keyword"
	ModeInventory  SearchMode = "inventory"
	ModeIndex      SearchMode = "index"
	ModeYear       SearchMode = "year"
	ModePublisher  SearchMode = "publisher"
	ModeSubject    SearchMode = "subject"
	ModeLanguage   SearchMode = "language"
	ModeAll        SearchMode = "all"
)

// SearchQuery is a search request.
type SearchQuery struct {
	Expression  string
	Mode        SearchMode
	Value       string
	Limit       int
	Offset      int
	Format      string
	WithRecords bool
	MinMFN      int
	MaxMFN      int
	Sequential  string
}

// SearchHit is one found item.
type SearchHit struct {
	MFN         int     `json:"mfn"`
	Description string  `json:"description,omitempty"`
	Record      *Record `json:"record,omitempty"`
}

// SearchResult is a search page.
type SearchResult struct {
	Expression string      `json:"expression"`
	Count      int         `json:"count"`
	Total      int         `json:"total,omitempty"`
	Hits       []SearchHit `json:"hits"`
}

// TermInfo is a dictionary term.
type TermInfo struct {
	Text  string `json:"text"`
	Count int    `json:"count"`
}
