package json

import (
	messages "github.com/cucumber/cucumber-messages-go/v7"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProcessTestStepFinished", func() {
	var (
		lookup *MessageLookup
	)

	BeforeEach(func() {
		lookup = &MessageLookup{}
		lookup.Initialize(false)
	})

	It("returns nil if the step does not exist", func() {
		testStepFinished := &messages.TestStepFinished{
			TestStepId: "unknown-step",
		}
		testStep := ProcessTestStepFinished(testStepFinished, lookup)

		Expect(testStep).To(BeNil())
	})

	Context("When step references a Hook", func() {
		BeforeEach(func() {
			lookup.ProcessMessage(
				makeTestCaseHookDefinitionConfigEnvelope(
					&messages.TestCaseHookDefinitionConfig{
						Id: "hook-id",
					},
				),
			)

			lookup.ProcessMessage(
				makeTestCaseEnvelope(
					makeTestCase(
						"test-case-id",
						"whatever-pickle-id",
						[]*messages.TestCase_TestStep{
							makeHookTestStep("hook-step-id", "hook-id"),
							makeHookTestStep("wrong-hook-step-id", "unknown-hook-id"),
						},
					),
				),
			)
		})

		It("returns a TestStep including the TestCaseHookDefinitionConfig", func() {
			testStepFinished := &messages.TestStepFinished{
				TestStepId: "hook-step-id",
				TestResult: &messages.TestResult{
					Status: messages.TestResult_PASSED,
				},
			}

			testStep := ProcessTestStepFinished(testStepFinished, lookup)

			Expect(testStep.Hook.Id).To(Equal("hook-id"))
			Expect(testStep.Result.Status).To(Equal(messages.TestResult_PASSED))
		})

		It("returns a TestStep with a nil Step", func() {
			testStepFinished := &messages.TestStepFinished{
				TestStepId: "hook-step-id",
			}

			testStep := ProcessTestStepFinished(testStepFinished, lookup)

			Expect(testStep.Step).To(BeNil())
		})

		It("returns nil if the Hook does not exist", func() {
			testStepFinished := &messages.TestStepFinished{
				TestStepId: "wrong-hook-step-id",
			}
			testStep := ProcessTestStepFinished(testStepFinished, lookup)

			Expect(testStep).To(BeNil())
		})
	})

	Context("When step references a PickleStep", func() {
		BeforeEach(func() {
			// This is a bit dirty hack to avoid creating all the AST
			step := makeGherkinStep("step-id", "Given", "a passed step")
			scenario := makeScenario("scenario-id", []*messages.GherkinDocument_Feature_Step{
				step,
			})
			lookup.stepByID[step.Id] = step
			lookup.scenarioByID[scenario.Id] = scenario

			lookup.ProcessMessage(&messages.Envelope{
				Message: &messages.Envelope_StepDefinitionConfig{
					StepDefinitionConfig: &messages.StepDefinitionConfig{
						Id: "step-def-id",
						Pattern: &messages.StepDefinitionPattern{
							Source: "a passed {word}",
						},
					},
				},
			})

			pickleStep := &messages.Pickle_PickleStep{
				Id:        "pickle-step-id",
				SourceIds: []string{step.Id},
				Text:      "a passed step",
			}

			pickle := &messages.Pickle{
				Id:        "pickle-id",
				Uri:       "some_feature.feature",
				SourceIds: []string{scenario.Id},
				Steps: []*messages.Pickle_PickleStep{
					pickleStep,
				},
			}

			lookup.ProcessMessage(
				makePickleEnvelope(pickle),
			)

			lookup.ProcessMessage(
				makeTestCaseEnvelope(
					makeTestCase(
						"test-case-id",
						pickle.Id,
						[]*messages.TestCase_TestStep{
							makeTestStep("test-step-id", "pickle-step-id", []string{"step-def-id"}),
							makeTestStep("unknown-pickle", "unknown-pickle-step-id", []string{}),
						},
					),
				),
			)

			lookup.ProcessMessage(&messages.Envelope{
				Message: &messages.Envelope_TestCaseStarted{
					TestCaseStarted: &messages.TestCaseStarted{
						Id:         "test-case-started-id",
						TestCaseId: "test-case-id",
					},
				},
			})
		})

		It("returns a TestStep including the FeatureStep", func() {
			testStepFinished := &messages.TestStepFinished{
				TestStepId:        "test-step-id",
				TestCaseStartedId: "test-case-started-id",
			}

			testStep := ProcessTestStepFinished(testStepFinished, lookup)
			Expect(testStep.Step.Id).To(Equal("step-id"))
		})

		It("returns a Step including the StepDefinitions", func() {
			testStepFinished := &messages.TestStepFinished{
				TestStepId:        "test-step-id",
				TestCaseStartedId: "test-case-started-id",
			}
			testStep := ProcessTestStepFinished(testStepFinished, lookup)
			Expect(len(testStep.StepDefinitions)).To(Equal(1))
			Expect(testStep.StepDefinitions[0].Pattern.Source).To(Equal("a passed {word}"))
		})

		It("Returns Nil if the pickle step is unknown", func() {
			testStepFinished := &messages.TestStepFinished{
				TestStepId:        "unknown-pickle",
				TestCaseStartedId: "test-case-started-id",
			}

			testStep := ProcessTestStepFinished(testStepFinished, lookup)
			Expect(testStep).To(BeNil())
		})
	})
})

var _ = Describe("TestStepToJSON", func() {
	var (
		step     *TestStep
		jsonStep *jsonStep
	)

	Context("When TestStep comes from a Hook", func() {
		BeforeEach(func() {
			step = &TestStep{
				Hook: &messages.TestCaseHookDefinitionConfig{
					Location: &messages.SourceReference{
						Uri: "some/hooks.go",
						Location: &messages.Location{
							Column: 3,
							Line:   12,
						},
					},
				},
				Result: &messages.TestResult{
					Status: messages.TestResult_PASSED,
				},
			}
			jsonStep = TestStepToJSON(step)
		})

		It("Has a Match", func() {
			Expect(jsonStep.Match.Location).To(Equal("some/hooks.go:12"))
		})

		It("Has a Result", func() {
			Expect(jsonStep.Result.Status).To(Equal("passed"))
		})
	})

	Context("When TestStep comes from a feature step", func() {
		BeforeEach(func() {
			step = &TestStep{
				Step: &messages.GherkinDocument_Feature_Step{
					Id:      "some-id",
					Keyword: "Given",
					Text:    "a <status> step",
					Location: &messages.Location{
						Line: 5,
					},
				},
				Pickle: &messages.Pickle{
					Uri: "my_feature.feature",
				},
				PickleStep: &messages.Pickle_PickleStep{
					Text: "a passed step",
				},
				Result: &messages.TestResult{
					Status: messages.TestResult_FAILED,
				},
			}
			jsonStep = TestStepToJSON(step)
		})

		It("gets keyword from Step", func() {
			Expect(jsonStep.Keyword).To(Equal("Given"))
		})

		It("gets name from PickleStep", func() {
			Expect(jsonStep.Name).To(Equal("a passed step"))
		})

		It("has a Result", func() {
			Expect(jsonStep.Result.Status).To(Equal("failed"))
		})

		It("has a Line", func() {
			Expect(jsonStep.Line).To(Equal(uint32(5)))
		})

		Context("When it does not have a StepDefinition", func() {
			It("Has a match referencing the feature file", func() {
				Expect(jsonStep.Match.Location).To(Equal("my_feature.feature:5"))
			})
		})

		Context("When it has a StepDefinition", func() {
			It("Has a match referencing the feature file", func() {
				step.StepDefinitions = []*messages.StepDefinitionConfig{
					&messages.StepDefinitionConfig{
						Location: &messages.SourceReference{
							Uri: "support_code.go",
							Location: &messages.Location{
								Line: 12,
							},
						},
					},
				}

				jsonStep = TestStepToJSON(step)
				Expect(jsonStep.Match.Location).To(Equal("support_code.go:12"))
			})
		})
	})
})
