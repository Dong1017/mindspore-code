package askuserquestion

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gitcode.com/mindspore/mscli/tools"
)

type stubPromptUI struct {
	resp PromptResponse
	err  error
	req  PromptRequest
}

func (s *stubPromptUI) Ask(_ context.Context, req PromptRequest) (PromptResponse, error) {
	s.req = req
	return s.resp, s.err
}

func TestToolSchema_ContainsNestedQuestionShapeAndGuidance(t *testing.T) {
	tool := NewTool(nil)
	schema := tool.Schema()

	questions, ok := schema.Properties["questions"]
	if !ok {
		t.Fatal("questions property missing")
	}
	if questions.Type != "array" {
		t.Fatalf("questions.Type = %q, want array", questions.Type)
	}
	if questions.Items == nil {
		t.Fatal("questions.Items = nil, want nested question schema")
	}

	questionSchema := questions.Items
	options := questionSchema.Properties["options"]
	if options.Items == nil {
		t.Fatal("options.Items = nil, want nested option schema")
	}
	for _, want := range []string{"Two to four", "Other", "manual input", "custom input"} {
		if !strings.Contains(options.Description, want) {
			t.Fatalf("options.Description = %q, want substring %q", options.Description, want)
		}
	}
	if got := questionSchema.Properties["multiSelect"].Description; !strings.Contains(got, "multiple options") {
		t.Fatalf("multiSelect description = %q, want multiple-options guidance", got)
	}
	if got := options.Items.Properties["label"].Description; !strings.Contains(got, "(Recommended)") {
		t.Fatalf("label description = %q, want recommended guidance", got)
	}
	if !strings.Contains(questions.Description, "finalized plan") {
		t.Fatalf("questions.Description = %q, want plan approval guidance", questions.Description)
	}
	if description := tool.Description(); !strings.Contains(description, "two to four concrete options") || !strings.Contains(description, "Other/manual/custom-input") {
		t.Fatalf("Description() = %q, want option count and custom-input guidance", description)
	}
}

func TestToolExecute_ReturnsCollectedAnswers(t *testing.T) {
	ui := &stubPromptUI{
		resp: PromptResponse{
			Answers: []PromptAnswer{
				{Question: "Which scope should we optimize first?", Answer: "backend"},
				{Question: "Which tests do you want?", Answer: "unit, integration"},
			},
		},
	}
	tool := NewTool(ui)
	params := mustJSON(t, PromptRequest{
		Questions: []Question{
			{
				Header:   "Scope",
				Question: "Which scope should we optimize first?",
				Options: []QuestionOption{
					{Label: "backend", Description: "Optimize backend first"},
					{Label: "frontend", Description: "Optimize frontend first"},
				},
			},
			{
				Header:      "Tests",
				Question:    "Which tests do you want?",
				MultiSelect: true,
				Options: []QuestionOption{
					{Label: "unit", Description: "Add unit tests"},
					{Label: "integration", Description: "Add integration tests"},
				},
			},
		},
	})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute() err = %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Execute() result.Error = %v", result.Error)
	}
	if got, want := result.Summary, "2 answers collected"; got != want {
		t.Fatalf("result.Summary = %q, want %q", got, want)
	}
	if !strings.Contains(result.Content, `"Which scope should we optimize first?" = "backend"`) {
		t.Fatalf("result.Content missing first answer:\n%s", result.Content)
	}
	if len(ui.req.Questions) != 2 {
		t.Fatalf("prompt ui saw %d questions, want 2", len(ui.req.Questions))
	}
}

func TestToolExecute_Declined(t *testing.T) {
	tool := NewTool(&stubPromptUI{resp: PromptResponse{Declined: true}})
	params := mustJSON(t, PromptRequest{Questions: []Question{validQuestion("Which scope should we optimize first?")}})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute() err = %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Execute() result.Error = %v", result.Error)
	}
	if got, want := result.Summary, "declined"; got != want {
		t.Fatalf("result.Summary = %q, want %q", got, want)
	}
}

func TestToolExecute_ValidatesRequest(t *testing.T) {
	cases := []struct {
		name string
		req  PromptRequest
		want string
	}{
		{
			name: "no questions",
			req:  PromptRequest{},
			want: "questions must contain 1 to 4 entries",
		},
		{
			name: "too many questions",
			req: PromptRequest{Questions: []Question{
				validQuestion("Question 1?"), validQuestion("Question 2?"), validQuestion("Question 3?"), validQuestion("Question 4?"), validQuestion("Question 5?"),
			}},
			want: "questions must contain 1 to 4 entries",
		},
		{
			name: "empty header",
			req: PromptRequest{Questions: []Question{{
				Question: "Which scope?",
				Options:  validOptions(),
			}}},
			want: "questions[0].header is required",
		},
		{
			name: "empty question",
			req: PromptRequest{Questions: []Question{{
				Header:  "Scope",
				Options: validOptions(),
			}}},
			want: "questions[0].question is required",
		},
		{
			name: "duplicate question",
			req: PromptRequest{Questions: []Question{
				validQuestion("Which scope?"), validQuestion("Which scope?"),
			}},
			want: "question text must be unique",
		},
		{
			name: "one option",
			req: PromptRequest{Questions: []Question{{
				Header:   "Scope",
				Question: "Which scope?",
				Options:  []QuestionOption{{Label: "backend", Description: "Optimize backend"}},
			}}},
			want: "questions[0].options must contain 2 to 4 concrete entries",
		},
		{
			name: "too many options",
			req: PromptRequest{Questions: []Question{{
				Header:   "Scope",
				Question: "Which scope?",
				Options: []QuestionOption{
					{Label: "backend", Description: "Optimize backend"},
					{Label: "frontend", Description: "Optimize frontend"},
					{Label: "tests", Description: "Optimize tests"},
					{Label: "docs", Description: "Optimize docs"},
					{Label: "infra", Description: "Optimize infra"},
				},
			}}},
			want: "questions[0].options must contain 2 to 4 concrete entries",
		},
		{
			name: "empty option label",
			req: PromptRequest{Questions: []Question{{
				Header:   "Scope",
				Question: "Which scope?",
				Options: []QuestionOption{
					{Label: "", Description: "Optimize backend"},
					{Label: "frontend", Description: "Optimize frontend"},
				},
			}}},
			want: "questions[0].options[0].label is required",
		},
		{
			name: "empty option description",
			req: PromptRequest{Questions: []Question{{
				Header:   "Scope",
				Question: "Which scope?",
				Options: []QuestionOption{
					{Label: "backend", Description: ""},
					{Label: "frontend", Description: "Optimize frontend"},
				},
			}}},
			want: "questions[0].options[0].description is required",
		},
		{
			name: "duplicate option label",
			req: PromptRequest{Questions: []Question{{
				Header:   "Scope",
				Question: "Which scope?",
				Options: []QuestionOption{
					{Label: "backend", Description: "Optimize backend"},
					{Label: "backend", Description: "Optimize backend again"},
				},
			}}},
			want: "option labels must be unique",
		},
	}

	tool := NewTool(&stubPromptUI{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), mustJSON(t, tc.req))
			if err != nil {
				t.Fatalf("Execute() err = %v", err)
			}
			if result.Error == nil {
				t.Fatal("result.Error = nil, want validation error")
			}
			if !strings.Contains(result.Error.Error(), tc.want) {
				t.Fatalf("result.Error = %v, want substring %q", result.Error, tc.want)
			}
		})
	}
}

func TestToolExecute_StripsEnglishCustomInputOptions(t *testing.T) {
	cases := []string{"Other", "manual input", "manual entry", "custom input", "custom value"}
	for _, label := range cases {
		t.Run(label, func(t *testing.T) {
			ui := &stubPromptUI{resp: PromptResponse{Answers: []PromptAnswer{{Question: "Which scope?", Answer: "backend"}}}}
			tool := NewTool(ui)
			params := mustJSON(t, PromptRequest{Questions: []Question{{
				Header:   "Scope",
				Question: "Which scope?",
				Options: []QuestionOption{
					{Label: "backend", Description: "Optimize backend"},
					{Label: "frontend", Description: "Optimize frontend"},
					{Label: label, Description: "Fallback choice"},
				},
			}}})

			result, err := tool.Execute(context.Background(), params)
			if err != nil {
				t.Fatalf("Execute() err = %v", err)
			}
			if result.Error != nil {
				t.Fatalf("Execute() result.Error = %v", result.Error)
			}
			if got := len(ui.req.Questions[0].Options); got != 2 {
				t.Fatalf("normalized option count = %d, want 2", got)
			}
		})
	}
}

func TestToolExecute_FailsWhenNormalizationLeavesOneConcreteOption(t *testing.T) {
	tool := NewTool(&stubPromptUI{})
	params := mustJSON(t, PromptRequest{Questions: []Question{{
		Header:   "Scope",
		Question: "Which scope?",
		Options: []QuestionOption{
			{Label: "backend", Description: "Optimize backend"},
			{Label: "Other", Description: "Fallback choice"},
		},
	}}})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute() err = %v", err)
	}
	if result.Error == nil {
		t.Fatal("result.Error = nil, want validation error")
	}
	if !strings.Contains(result.Error.Error(), "2 to 4 concrete entries") {
		t.Fatalf("result.Error = %v, want 2-4 concrete entries error", result.Error)
	}
}

func TestToolExecute_DoesNotStripLegitimateCustomLikeLabels(t *testing.T) {
	ui := &stubPromptUI{resp: PromptResponse{Answers: []PromptAnswer{{Question: "Which profile?", Answer: "custom_path"}}}}
	tool := NewTool(ui)
	params := mustJSON(t, PromptRequest{Questions: []Question{{
		Header:   "Profile",
		Question: "Which profile?",
		Options: []QuestionOption{
			{Label: "custom_path", Description: "Use the configured path."},
			{Label: "custom profile", Description: "Use the named profile."},
			{Label: "custom", Description: "Product-specific setting."},
			{Label: "backend", Description: "This is not a custom input option."},
		},
	}}})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute() err = %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Execute() result.Error = %v", result.Error)
	}
	if got := len(ui.req.Questions[0].Options); got != 4 {
		t.Fatalf("normalized option count = %d, want 4", got)
	}
}

func TestToolMetadata(t *testing.T) {
	tool := NewTool(nil)
	interactive, ok := any(tool).(tools.InteractiveTool)
	if !ok {
		t.Fatal("Tool does not implement tools.InteractiveTool")
	}
	if !interactive.RequiresUserInteraction() {
		t.Fatal("RequiresUserInteraction() = false, want true")
	}
	readOnly, ok := any(tool).(tools.ReadOnlyTool)
	if !ok {
		t.Fatal("Tool does not implement tools.ReadOnlyTool")
	}
	if !readOnly.IsReadOnly() {
		t.Fatal("IsReadOnly() = false, want true")
	}
	concurrencySafe, ok := any(tool).(tools.ConcurrencySafeTool)
	if !ok {
		t.Fatal("Tool does not implement tools.ConcurrencySafeTool")
	}
	if !concurrencySafe.IsConcurrencySafe() {
		t.Fatal("IsConcurrencySafe() = false, want true")
	}
}

func validQuestion(question string) Question {
	return Question{
		Header:   "Scope",
		Question: question,
		Options:  validOptions(),
	}
}

func validOptions() []QuestionOption {
	return []QuestionOption{
		{Label: "backend", Description: "Optimize backend"},
		{Label: "frontend", Description: "Optimize frontend"},
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() err = %v", err)
	}
	return data
}
