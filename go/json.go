package json

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	messages "github.com/cucumber/cucumber-messages-go/v7"
	gio "github.com/gogo/protobuf/io"
)

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

type Formatter struct {
	lookup *MessageLookup

	jsonFeatures            []*jsonFeature
	jsonFeaturesByURI       map[string]*jsonFeature
	jsonStepsByPickleStepId map[string]*jsonStep
	exampleRowIndexById     map[string]int
	verbose                 bool
}

// ProcessMessages writes a JSON report to STDOUT
func (self *Formatter) ProcessMessages(reader gio.ReadCloser, stdout io.Writer) (err error) {
	self.verbose = false
	self.lookup = &MessageLookup{}
	self.lookup.Initialize(self.verbose)

	self.jsonFeatures = make([]*jsonFeature, 0)
	self.jsonFeaturesByURI = make(map[string]*jsonFeature)
	self.jsonStepsByPickleStepId = make(map[string]*jsonStep)
	self.exampleRowIndexById = make(map[string]int)

	for {
		envelope := &messages.Envelope{}
		err := reader.ReadMsg(envelope)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = self.lookup.ProcessMessage(envelope)
		if err != nil {
			return err
		}

		switch m := envelope.Message.(type) {
		case *messages.Envelope_GherkinDocument:
			self.comment("Treating GherkinDocument")
			for _, child := range m.GherkinDocument.Feature.Children {
				scenario := child.GetScenario()
				if scenario != nil {
					for _, example := range scenario.Examples {
						for index, row := range example.TableBody {
							// index + 2: it's a 1 based index and the header is counted too.
							self.exampleRowIndexById[row.Id] = index + 2
						}
					}
				}
			}

		case *messages.Envelope_Pickle:
			self.comment(fmt.Sprintf(
				"Treating Pickle: %s - %s",
				m.Pickle.Id,
				m.Pickle.SourceIds,
			))

			pickle := m.Pickle
			jsonFeature := self.findOrCreateJsonFeature(pickle)
			scenario := self.lookup.LookupScenario(pickle.SourceIds[0])
			elementLine := scenario.Location.Line

			// TODO: find a better way to get backgrounds
			background := &messages.GherkinDocument_Feature_Background{}
			scenarioJsonSteps := make([]*jsonStep, 0)
			backgroundJsonSteps := make([]*jsonStep, 0)

			for _, pickleStep := range pickle.Steps {
				step := self.lookup.LookupStep(pickleStep.SourceIds[0])
				jsonStep := &jsonStep{
					Keyword: step.Keyword,
					Line:    step.Location.Line,
					Name:    pickleStep.Text,
					// The match field defaults to the Gherkin step itself for some curious reason
					Match: &jsonStepMatch{
						Location: fmt.Sprintf("%s", pickle.Uri),
					},
				}

				docString := step.GetDocString()
				if docString != nil {
					jsonStep.DocString = &jsonDocString{
						Line:        docString.Location.Line,
						ContentType: docString.ContentType,
						Value:       docString.Content,
					}
				}

				datatable := step.GetDataTable()
				if datatable != nil {
					jsonStep.Rows = make([]*jsonDatatableRow, len(datatable.GetRows()))
					for rowIndex, row := range datatable.GetRows() {
						cells := make([]string, len(row.Cells))
						for cellIndex, cell := range row.Cells {
							cells[cellIndex] = cell.Value
						}

						jsonStep.Rows[rowIndex] = &jsonDatatableRow{
							Cells: cells,
						}
					}
				}
				if self.isBackgroundStep(step.Id) {
					backgroundJsonSteps = append(backgroundJsonSteps, jsonStep)
					background = self.lookup.LookupBrackgroundByStepId(step.Id)
				} else {
					scenarioJsonSteps = append(scenarioJsonSteps, jsonStep)
				}

				self.jsonStepsByPickleStepId[pickleStep.Id] = jsonStep
			}

			if len(backgroundJsonSteps) > 0 {
				jsonFeature.Elements = append(jsonFeature.Elements, &jsonFeatureElement{
					Description: background.Description,
					Keyword:     background.Keyword,
					Line:        background.Location.Line,
					Steps:       backgroundJsonSteps,
					Type:        "background",
				})
			}

			scenarioID := fmt.Sprintf("%s;%s", jsonFeature.ID, self.makeId(scenario.Name))

			if len(pickle.SourceIds) > 1 {
				exampleRow := self.lookup.LookupExampleRow(pickle.SourceIds[1])
				example := self.lookup.LookupExample(pickle.SourceIds[1])
				scenarioID = fmt.Sprintf(
					"%s;%s;%s;%d",
					jsonFeature.ID,
					self.makeId(scenario.Name),
					self.makeId(example.Name),
					self.exampleRowIndexById[exampleRow.Id])

				elementLine = exampleRow.Location.Line
			}

			scenarioTags := make([]*jsonTag, len(pickle.Tags))
			for tagIndex, pickleTag := range pickle.Tags {
				tag := self.lookup.LookupTagByID(pickleTag.SourceId)

				scenarioTags[tagIndex] = &jsonTag{
					Line: tag.Location.Line,
					Name: tag.Name,
				}
			}

			jsonFeature.Elements = append(jsonFeature.Elements, &jsonFeatureElement{
				Description: scenario.Description,
				ID:          scenarioID,
				Keyword:     scenario.Keyword,
				Line:        elementLine,
				Name:        scenario.Name,
				Steps:       scenarioJsonSteps,
				Type:        "scenario",
				Tags:        scenarioTags,
			})

		case *messages.Envelope_TestStepFinished:
			testStep := self.lookup.LookupTestStepByID(m.TestStepFinished.TestStepId)
			pickleStep := self.lookup.LookupPickleStepByID(testStep.PickleStepId)
			jsonStep := self.jsonStepsByPickleStepId[pickleStep.Id]

			status := strings.ToLower(m.TestStepFinished.TestResult.Status.String())
			jsonStep.Result = &jsonStepResult{
				Status:       status,
				ErrorMessage: m.TestStepFinished.TestResult.Message,
			}
			if m.TestStepFinished.TestResult.Duration != nil {
				duration := messages.DurationToGoDuration(*m.TestStepFinished.TestResult.Duration)
				// Go's time.Duration is an int64 representing nanoseconds
				jsonStep.Result.Duration = uint64(duration)
			}

			stepDefinitions := self.lookup.LookupStepDefinitionConfigsByIDs(testStep.StepDefinitionId)
			if len(stepDefinitions) > 0 {
				jsonStep.Match = &jsonStepMatch{
					Location: fmt.Sprintf(
						"%s:%d",
						stepDefinitions[0].Location.Uri,
						stepDefinitions[0].Location.Location.Line,
					),
				}
			}
		}
	}

	output, _ := json.MarshalIndent(self.jsonFeatures, "", "  ")
	_, err = fmt.Fprintln(stdout, string(output))
	return err
}

func (self *Formatter) findOrCreateJsonFeature(pickle *messages.Pickle) *jsonFeature {
	jFeature, ok := self.jsonFeaturesByURI[pickle.Uri]
	if !ok {
		gherkinDocumentFeature := self.lookup.LookupGherkinDocument(pickle.Uri).Feature

		jFeature = &jsonFeature{
			Description: gherkinDocumentFeature.Description,
			Elements:    make([]*jsonFeatureElement, 0),
			ID:          self.makeId(gherkinDocumentFeature.Name),
			Keyword:     gherkinDocumentFeature.Keyword,
			Line:        gherkinDocumentFeature.Location.Line,
			Name:        gherkinDocumentFeature.Name,
			URI:         pickle.Uri,
			Tags:        make([]*jsonTag, len(gherkinDocumentFeature.Tags)),
		}

		for tagIndex, tag := range gherkinDocumentFeature.Tags {
			jFeature.Tags[tagIndex] = &jsonTag{
				Line: tag.Location.Line,
				Name: tag.Name,
			}
		}

		self.jsonFeaturesByURI[pickle.Uri] = jFeature
		self.jsonFeatures = append(self.jsonFeatures, jFeature)
	}
	return jFeature
}

func (self *Formatter) isBackgroundStep(id string) bool {
	_, ok := self.lookup.backgroundByStepId[id]
	return ok
}

func (self *Formatter) makeId(s string) string {
	return strings.ToLower(strings.Replace(s, " ", "-", -1))
}

func (self *Formatter) comment(message string) {
	if self.verbose {
		fmt.Println(fmt.Sprintf("// Formatter: %s", message))
	}
}
