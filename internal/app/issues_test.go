package app

import (
	"strings"
	"testing"
	"time"

	issuepkg "github.com/mindspore-lab/mindspore-cli/internal/issues"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestCmdIssuesDefaultsToAllAndOpensIssueIndexView(t *testing.T) {
	store := &fakeAppIssueStore{
		issues: []issuepkg.Issue{
			{ID: 42, Key: "ISSUE-42", Title: "acc failure in migrate", Kind: issuepkg.KindAccuracy, Status: "ready", Reporter: "alice", UpdatedAt: time.Now()},
		},
	}
	app := &Application{
		EventCh:      make(chan model.Event, 4),
		issueService: issuepkg.NewService(store),
	}

	app.cmdIssues(nil)

	ev := <-app.EventCh
	if ev.Type != model.IssueIndexOpen {
		t.Fatalf("event type = %s, want %s", ev.Type, model.IssueIndexOpen)
	}
	if ev.IssueView == nil || ev.IssueView.Filter != "all" {
		t.Fatalf("issue view filter = %#v, want all", ev.IssueView)
	}
	if got := len(ev.IssueView.Items); got != 1 {
		t.Fatalf("issue count = %d, want 1", got)
	}
}

func TestCmdIssueReportInputCreatesIssue(t *testing.T) {
	store := &fakeAppIssueStore{}
	app := &Application{
		EventCh:      make(chan model.Event, 4),
		issueService: issuepkg.NewService(store),
		issueUser:    "alice",
	}

	app.cmdFeedbackIssue("accuracy acc failure in migrate")

	ev := <-app.EventCh
	if ev.Type != model.AgentReply {
		t.Fatalf("event type = %s, want %s", ev.Type, model.AgentReply)
	}
	if got, want := store.lastCreateKind, issuepkg.KindAccuracy; got != want {
		t.Fatalf("kind = %q, want %q", got, want)
	}
	if got, want := store.lastCreateTitle, "acc failure in migrate"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestParseIssueCommandTargetTreatsFreeTextAsPrompt(t *testing.T) {
	target, err := parseIssueCommandTarget("training loss too big", "/diagnose")
	if err != nil {
		t.Fatalf("parseIssueCommandTarget returned error: %v", err)
	}
	if target.HasIssue {
		t.Fatalf("HasIssue = true, want false")
	}
	if got, want := target.Prompt, "training loss too big"; got != want {
		t.Fatalf("Prompt = %q, want %q", got, want)
	}
}

func TestParseIssueCommandTargetAcceptsIssueKeyWithExtraContext(t *testing.T) {
	target, err := parseIssueCommandTarget("ISSUE-42 training loss too big", "/diagnose")
	if err != nil {
		t.Fatalf("parseIssueCommandTarget returned error: %v", err)
	}
	if !target.HasIssue {
		t.Fatalf("HasIssue = false, want true")
	}
	if got, want := target.IssueID, 42; got != want {
		t.Fatalf("IssueID = %d, want %d", got, want)
	}
	if got, want := target.Prompt, "training loss too big"; got != want {
		t.Fatalf("Prompt = %q, want %q", got, want)
	}
}

func TestParseIssueCommandTargetRejectsMalformedIssueKey(t *testing.T) {
	_, err := parseIssueCommandTarget("ISSUE-abc", "/fix")
	if err == nil {
		t.Fatal("parseIssueCommandTarget error = nil, want error")
	}
	if got, want := err.Error(), "invalid issue id"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestBuildSkillTaskFreeTextDiagnose(t *testing.T) {
	app := &Application{}
	target := issueCommandTarget{Prompt: "training crashes on Ascend"}
	task, err := app.buildSkillTask(target, "diagnose")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(task, "diagnose mode") {
		t.Fatalf("task should contain diagnose mode, got %q", task)
	}
	if !strings.Contains(task, "training crashes on Ascend") {
		t.Fatalf("task should contain user problem, got %q", task)
	}
}

func TestBuildSkillTaskFreeTextFix(t *testing.T) {
	app := &Application{}
	target := issueCommandTarget{Prompt: "accuracy dropped after migration"}
	task, err := app.buildSkillTask(target, "fix")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(task, "fix mode") {
		t.Fatalf("task should contain fix mode, got %q", task)
	}
	if !strings.Contains(task, "accuracy dropped after migration") {
		t.Fatalf("task should contain user problem, got %q", task)
	}
}

func TestBuildSkillTaskIssueTargetUsesIssueKind(t *testing.T) {
	store := &fakeAppIssueStore{
		issue: &issuepkg.Issue{
			ID: 42, Key: "ISSUE-42", Title: "throughput too low on NPU",
			Kind: issuepkg.KindPerformance, Summary: "NPU utilization at 30%",
		},
		notes: []issuepkg.Note{
			{Author: "alice", Content: "profiler shows idle cycles"},
		},
	}
	app := &Application{
		issueService: issuepkg.NewService(store),
	}

	target := issueCommandTarget{HasIssue: true, IssueID: 42, Prompt: "extra info"}
	task, err := app.buildSkillTask(target, "diagnose")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(task, "performance-agent") {
		t.Fatalf("task should route to performance-agent, got %q", task)
	}
	if !strings.Contains(task, "ISSUE-42") {
		t.Fatalf("task should contain issue key, got %q", task)
	}
	if !strings.Contains(task, "throughput too low on NPU") {
		t.Fatalf("task should contain issue title, got %q", task)
	}
	if !strings.Contains(task, "NPU utilization at 30%") {
		t.Fatalf("task should contain issue summary, got %q", task)
	}
	if !strings.Contains(task, "profiler shows idle cycles") {
		t.Fatalf("task should contain issue notes, got %q", task)
	}
	if !strings.Contains(task, "extra info") {
		t.Fatalf("task should contain additional context, got %q", task)
	}
}

func TestBuildSkillTaskIssueTargetFailureKind(t *testing.T) {
	store := &fakeAppIssueStore{
		issue: &issuepkg.Issue{
			ID: 1, Key: "ISSUE-1", Title: "OOM on training",
			Kind: issuepkg.KindFailure,
		},
	}
	app := &Application{
		issueService: issuepkg.NewService(store),
	}

	target := issueCommandTarget{HasIssue: true, IssueID: 1}
	task, err := app.buildSkillTask(target, "fix")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(task, "failure-agent") {
		t.Fatalf("task should route to failure-agent, got %q", task)
	}
	if !strings.Contains(task, "fix mode") {
		t.Fatalf("task should contain fix mode, got %q", task)
	}
}

func TestBuildDiagnosticContextExtractsConservativeSignals(t *testing.T) {
	ctx := buildDiagnosticContext(issueCommandTarget{Prompt: "ImportError: torch_npu runtime dependency missing on Ascend CANN while importing MindSpore"}, "")
	if ctx.Command != "/diagnose" {
		t.Fatalf("Command = %q, want /diagnose", ctx.Command)
	}
	if ctx.UserInput != "ImportError: torch_npu runtime dependency missing on Ascend CANN while importing MindSpore" {
		t.Fatalf("UserInput = %q", ctx.UserInput)
	}
	if ctx.Problem.InferredType != string(issuepkg.KindFailure) {
		t.Fatalf("InferredType = %q, want failure", ctx.Problem.InferredType)
	}
	if ctx.Problem.InferredStage != "import" {
		t.Fatalf("InferredStage = %q, want import", ctx.Problem.InferredStage)
	}
	if ctx.Signals.MainError == "" {
		t.Fatal("MainError is empty, want inferred error")
	}
	if !containsString(ctx.Signals.Keywords, "torch_npu") {
		t.Fatalf("Keywords = %#v, want torch_npu", ctx.Signals.Keywords)
	}
	if !containsString(ctx.Signals.StackKeywords, "importerror") {
		t.Fatalf("StackKeywords = %#v, want importerror", ctx.Signals.StackKeywords)
	}
	if ctx.Environment.Hardware.Accelerator != "ascend" {
		t.Fatalf("Accelerator = %q, want ascend", ctx.Environment.Hardware.Accelerator)
	}
	if len(ctx.Environment.Frameworks) == 0 || ctx.Environment.Frameworks[0].Name != "mindspore" {
		t.Fatalf("Frameworks = %#v, want mindspore", ctx.Environment.Frameworks)
	}
}

func TestBuildDiagnosticContextKeepsTorchFrameworkName(t *testing.T) {
	ctx := buildDiagnosticContext(issueCommandTarget{Prompt: "RuntimeError from torch on Ascend"}, "")
	if len(ctx.Environment.Frameworks) == 0 || ctx.Environment.Frameworks[0].Name != "torch" {
		t.Fatalf("Frameworks = %#v, want first framework torch", ctx.Environment.Frameworks)
	}
}

func TestBuildDiagnosticContextUsesTaskTextForIssueTarget(t *testing.T) {
	task := "Load skill failure-agent in diagnose mode.\n\nIssue: ISSUE-42 — ImportError torch_npu missing on Ascend\nKind: failure\nSummary: CANN import failure"
	ctx := buildDiagnosticContext(issueCommandTarget{HasIssue: true, IssueID: 42}, task)
	if !strings.Contains(ctx.UserInput, "ISSUE-42") {
		t.Fatalf("UserInput = %q, want issue text", ctx.UserInput)
	}
	if ctx.Problem.InferredType != string(issuepkg.KindFailure) {
		t.Fatalf("InferredType = %q, want failure", ctx.Problem.InferredType)
	}
	if ctx.Problem.InferredStage != "import" {
		t.Fatalf("InferredStage = %q, want import", ctx.Problem.InferredStage)
	}
	if ctx.Environment.Hardware.Accelerator != "ascend" {
		t.Fatalf("Accelerator = %q, want ascend", ctx.Environment.Hardware.Accelerator)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestEnrichDiagnoseTaskMissingPackReturnsOriginalTask(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	app := &Application{}
	task := "Load the appropriate diagnosis skill in diagnose mode."
	got := app.enrichDiagnoseTask(task, issueCommandTarget{Prompt: "ImportError: torch_npu runtime dependency missing"})
	if got != task {
		t.Fatalf("enriched task = %q, want original task", got)
	}
	if app.latestDiagnoseSummary == nil {
		t.Fatal("latestDiagnoseSummary = nil, want summary")
	}
	if len(app.latestDiagnoseSummary.FactoryHintsUsed) != 0 {
		t.Fatalf("FactoryHintsUsed = %d, want 0", len(app.latestDiagnoseSummary.FactoryHintsUsed))
	}
}

func TestCmdDiagnoseEmptyInputReturnsUsage(t *testing.T) {
	app := &Application{
		EventCh: make(chan model.Event, 4),
	}
	app.cmdDiagnose("")
	ev := <-app.EventCh
	if ev.Type != model.AgentReply {
		t.Fatalf("event type = %s, want %s", ev.Type, model.AgentReply)
	}
	if !strings.Contains(ev.Message, "Usage:") {
		t.Fatalf("message should contain Usage, got %q", ev.Message)
	}
}

func TestCmdFixEmptyInputReturnsUsage(t *testing.T) {
	app := &Application{
		EventCh: make(chan model.Event, 4),
	}
	app.cmdFix("")
	ev := <-app.EventCh
	if ev.Type != model.AgentReply {
		t.Fatalf("event type = %s, want %s", ev.Type, model.AgentReply)
	}
	if !strings.Contains(ev.Message, "Usage:") {
		t.Fatalf("message should contain Usage, got %q", ev.Message)
	}
}

type fakeAppIssueStore struct {
	lastCreateTitle string
	lastCreateKind  issuepkg.Kind
	issues          []issuepkg.Issue
	issue           *issuepkg.Issue
	notes           []issuepkg.Note
	activity        []issuepkg.Activity
}

func (f *fakeAppIssueStore) CreateIssue(title string, kind issuepkg.Kind, reporter string) (*issuepkg.Issue, error) {
	f.lastCreateTitle = title
	f.lastCreateKind = kind
	return &issuepkg.Issue{ID: 42, Key: "ISSUE-42", Title: title, Kind: kind, Status: "ready", Reporter: reporter}, nil
}

func (f *fakeAppIssueStore) ListIssues(status string) ([]issuepkg.Issue, error) {
	return f.issues, nil
}

func (f *fakeAppIssueStore) GetIssue(id int) (*issuepkg.Issue, error) {
	return f.issue, nil
}

func (f *fakeAppIssueStore) AddNote(issueID int, author, content string) (*issuepkg.Note, error) {
	return &issuepkg.Note{ID: 1, IssueID: issueID, Author: author, Content: content}, nil
}

func (f *fakeAppIssueStore) ListNotes(issueID int) ([]issuepkg.Note, error) {
	return f.notes, nil
}

func (f *fakeAppIssueStore) ListActivity(issueID int) ([]issuepkg.Activity, error) {
	return f.activity, nil
}

func (f *fakeAppIssueStore) ClaimIssue(id int, lead string) (*issuepkg.Issue, error) {
	return f.issue, nil
}

func (f *fakeAppIssueStore) UpdateStatus(id int, status string, actor string) (*issuepkg.Issue, error) {
	return f.issue, nil
}

func (f *fakeAppIssueStore) DockSummary() (*issuepkg.DockData, error) {
	return &issuepkg.DockData{}, nil
}
