package json

type jsonFeature struct {
	Description string                `json:"description"`
	Elements    []*jsonFeatureElement `json:"elements"`
	ID          string                `json:"id"`
	Keyword     string                `json:"keyword"`
	Line        uint32                `json:"line"`
	Name        string                `json:"name"`
	URI         string                `json:"uri"`
	Tags        []*jsonTag            `json:"tags,omitempty"`
}

type jsonFeatureElement struct {
	Description string      `json:"description"`
	ID          string      `json:"id,omitempty"`
	Keyword     string      `json:"keyword"`
	Line        uint32      `json:"line"`
	Name        string      `json:"name"`
	Steps       []*jsonStep `json:"steps"`
	Type        string      `json:"type"`
	Tags        []*jsonTag  `json:"tags,omitempty"`
}

type jsonStep struct {
	Keyword   string              `json:"keyword"`
	Line      uint32              `json:"line"`
	Name      string              `json:"name"`
	Result    *jsonStepResult     `json:"result"`
	Match     *jsonStepMatch      `json:"match,omitempty"`
	DocString *jsonDocString      `json:"doc_string,omitempty"`
	Rows      []*jsonDatatableRow `json:"rows,omitempty"`
}

type jsonDocString struct {
	ContentType string `json:"content_type"`
	Line        uint32 `json:"line"`
	Value       string `json:"value"`
}

type jsonDatatableRow struct {
	Cells []string `json:"cells"`
}

type jsonStepResult struct {
	Duration     uint64 `json:"duration,omitempty"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type jsonStepMatch struct {
	Location string `json:"location"`
}

type jsonTag struct {
	Line uint32 `json:"line"`
	Name string `json:"name"`
}
