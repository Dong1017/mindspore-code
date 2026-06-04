package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	issuepkg "github.com/mindspore-lab/mindspore-cli/internal/issues"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
	"github.com/mindspore-lab/mindspore-cli/ui/render"
)

func (a *Application) cmdFeedback(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /feedback [tags] <title> | /feedback acc|fail|perf <title>",
		}
		return
	}
	fields := strings.Fields(input)
	if _, err := issuepkg.NormalizeKind(fields[0]); err == nil {
		a.cmdFeedbackIssue(input)
	} else {
		a.cmdFeedbackBug(input)
	}
}

func (a *Application) cmdFeedbackBug(input string) {
	if !a.ensureIssueService() {
		return
	}

	title := strings.TrimSpace(input)
	if title == "" {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /feedback <title>"}
		return
	}
	issue, err := a.issueService.CreateIssue(title, issuepkg.KindBug, a.issueUser)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("report failed: %v", err)}
		return
	}
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("created %s [bug]: %s", issue.Key, issue.Title),
	}
}

func (a *Application) cmdNow() {
	if !a.ensureIssueService() {
		return
	}
	data, err := a.issueService.DockSummary()
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("dashboard failed: %v", err)}
		return
	}
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		RawANSI: true,
		Message: render.Dock(data),
	}
}

func (a *Application) cmdFeedbackIssue(input string) {
	if !a.ensureIssueService() {
		return
	}

	kind, title, err := parseIssueReportInput(input)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
		return
	}
	issue, err := a.issueService.CreateIssue(title, kind, a.issueUser)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("report failed: %v", err)}
		return
	}
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("%s created [%s]: %s", issue.Key, issue.Kind, issue.Title),
	}
}

func (a *Application) cmdIssues(args []string) {
	if !a.ensureIssueService() {
		return
	}
	status := "all"
	if len(args) > 0 {
		status = strings.ToLower(strings.TrimSpace(args[0]))
	}
	listStatus := status
	if status == "all" {
		listStatus = ""
	}
	issueList, err := a.issueService.ListIssues(listStatus)
	if err != nil {
		a.EventCh <- model.Event{
			Type: model.IssueIndexOpen,
			IssueView: &model.IssueEventData{
				Filter: status,
				Err:    err,
			},
		}
		return
	}
	a.EventCh <- model.Event{
		Type: model.IssueIndexOpen,
		IssueView: &model.IssueEventData{
			Filter: status,
			Items:  issueList,
		},
	}
}

func (a *Application) cmdIssueDetail(args []string) {
	if !a.ensureIssueService() {
		return
	}
	if len(args) == 0 {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /__issue_detail <issue-id>"}
		return
	}
	id, err := parseIssueRef(args[0])
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "invalid issue id"}
		return
	}
	a.emitIssueDetail(id, true)
}

func (a *Application) cmdIssueNoteInput(input string) {
	if !a.ensureIssueService() {
		return
	}
	ref, content, err := splitIssueNoteInput(input)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
		return
	}
	id, err := parseIssueRef(ref)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "invalid issue id"}
		return
	}
	if _, err := a.issueService.AddNote(id, a.issueUser, content); err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("add note failed: %v", err)}
		return
	}
	a.emitIssueDetail(id, true)
}

func (a *Application) cmdIssueClaim(args []string) {
	if !a.ensureIssueService() {
		return
	}
	if len(args) == 0 {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /__issue_claim <issue-id>"}
		return
	}
	id, err := parseIssueRef(args[0])
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "invalid issue id"}
		return
	}
	if _, err := a.issueService.ClaimIssue(id, a.issueUser); err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("claim issue failed: %v", err)}
		return
	}
	a.emitIssueDetail(id, true)
}

func (a *Application) cmdIssueStatus(args []string) {
	if !a.ensureIssueService() {
		return
	}
	if len(args) < 2 {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /status <ISSUE-id> <ready|doing|closed>"}
		return
	}
	id, err := parseIssueRef(args[0])
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "invalid issue id"}
		return
	}
	status, err := issuepkg.NormalizeStatus(args[1])
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
		return
	}
	if _, err := a.issueService.UpdateStatus(id, status, a.issueUser); err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("update issue status failed: %v", err)}
		return
	}
	a.emitIssueDetail(id, true)
}

func (a *Application) cmdDiagnose(input string) {
	a.runSkillCommand(input, "/diagnose")
}

func (a *Application) cmdFix(input string) {
	a.runSkillCommand(input, "/fix")
}

func (a *Application) cmdMigrate(input string) {
	task := fmt.Sprintf(
		"Load skill migrate-agent.\n\nUser request: %s",
		strings.TrimSpace(input),
	)
	a.EventCh <- model.Event{Type: model.AgentThinking}
	go a.runTask(task)
}

func (a *Application) cmdIntegrate(input string) {
	task := fmt.Sprintf(
		"Load the appropriate skill (algorithm-agent or operator-agent) based on the user request.\n\nUser request: %s",
		strings.TrimSpace(input),
	)
	a.EventCh <- model.Event{Type: model.AgentThinking}
	go a.runTask(task)
}

func (a *Application) cmdPreflight(input string) {
	task := "Load skill readiness-agent."
	if prompt := strings.TrimSpace(input); prompt != "" {
		task += "\n\nUser request: " + prompt
	}
	a.EventCh <- model.Event{Type: model.AgentThinking}
	go a.runTask(task)
}

func (a *Application) runSkillCommand(input, command string) {
	mode := strings.TrimPrefix(command, "/")
	target, err := parseIssueCommandTarget(input, command)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
		return
	}
	if target.HasIssue && !a.ensureIssueService() {
		return
	}

	task, err := a.buildSkillTask(target, mode)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
		return
	}
	if mode == "diagnose" {
		task = a.enrichDiagnoseTask(task, target)
	}
	if mode == "fix" {
		a.storeFixRunSummary(task, target)
	}

	a.EventCh <- model.Event{Type: model.AgentThinking}
	go a.runTask(task)
}

func (a *Application) enrichDiagnoseTask(task string, target issueCommandTarget) string {
	ctx := buildDiagnosticContext(target, task)
	enrichment, err := factoryruntime.BuildFactoryEnrichment(ctx, a.factoryPackLoadConfig())
	summary := factoryruntime.BuildDiagnoseRunSummary(ctx, enrichment.Matches)
	a.latestDiagnoseSummary = &summary
	a.latestRunKind = "diagnose"
	if err != nil || strings.TrimSpace(enrichment.HintBlock) == "" {
		return task
	}
	return task + "\n\n" + enrichment.HintBlock
}

func (a *Application) storeFixRunSummary(task string, target issueCommandTarget) {
	text := boundedDiagnosticText(firstNonEmptyText(target.Prompt, task))
	summary := factoryruntime.BuildFixRunSummary(factoryruntime.FixRunSummaryInput{
		Topic:              firstNonEmptyText(inferMainError(text), text),
		UserProblemSummary: text,
		PlannedFixSummary:  "Requested fix plan generated from bounded /fix input; execution results are not verified by this summary.",
		KeyEvidence:        fixSummaryEvidence(text),
	})
	a.latestFixSummary = &summary
	a.latestRunKind = "fix"
}

func fixSummaryEvidence(text string) []string {
	values := []string{inferMainError(text), inferProblemType(text), inferStage(text), inferAccelerator(text)}
	values = append(values, inferDiagnosticKeywords(text)...)
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func (a *Application) factoryPackLoadConfig() pack.LoadConfig {
	return pack.LoadConfig{}
}

func buildDiagnosticContext(target issueCommandTarget, taskText string) pack.DiagnosticContext {
	text := boundedDiagnosticText(firstNonEmptyText(target.Prompt, taskText))
	ctx := pack.DiagnosticContext{
		Command:   "/diagnose",
		UserInput: text,
	}
	ctx.Signals.MainError = inferMainError(text)
	ctx.Signals.Keywords = inferDiagnosticKeywords(text)
	ctx.Signals.StackKeywords = inferStackKeywords(text)
	ctx.Problem.InferredType = inferProblemType(text)
	ctx.Problem.InferredStage = inferStage(text)
	ctx.Environment.Frameworks = inferFrameworks(text)
	ctx.Environment.Hardware.Accelerator = inferAccelerator(text)
	return ctx
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boundedDiagnosticText(value string) string {
	fields := strings.Fields(value)
	if len(fields) <= 240 {
		return strings.TrimSpace(value)
	}
	return strings.Join(fields[:240], " ")
}

func inferMainError(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed != "" && (strings.Contains(lower, "error") || strings.Contains(lower, "exception") || strings.Contains(lower, "traceback") || strings.Contains(lower, "failed")) {
			return trimmed
		}
	}
	return ""
}

func inferDiagnosticKeywords(text string) []string {
	return presentSignals(text, []string{"torch_npu", "ascend", "cann", "mindspore", "importerror", "runtimeerror", "acl", "ge", "oom"}, 8)
}

func inferStackKeywords(text string) []string {
	return presentSignals(text, []string{"traceback", "importerror", "runtimeerror", "modulenotfounderror", "segmentation fault", "stack"}, 6)
}

func inferProblemType(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "accuracy") || strings.Contains(lower, "loss") || strings.Contains(lower, "nan") || strings.Contains(lower, "precision") {
		return string(issuepkg.KindAccuracy)
	}
	if strings.Contains(lower, "performance") || strings.Contains(lower, "throughput") || strings.Contains(lower, "latency") || strings.Contains(lower, "slow") {
		return string(issuepkg.KindPerformance)
	}
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "exception") || strings.Contains(lower, "crash") || strings.Contains(lower, "oom") {
		return string(issuepkg.KindFailure)
	}
	return ""
}

func inferStage(text string) string {
	lower := strings.ToLower(text)
	stageSignals := []struct {
		stage   string
		signals []string
	}{
		{stage: "import", signals: []string{"importerror", "import ", "module not found", "modulenotfounderror"}},
		{stage: "compile", signals: []string{"compile", "graph compile", "build graph"}},
		{stage: "train", signals: []string{"train", "training", "loss", "backward"}},
		{stage: "eval", signals: []string{"eval", "evaluation", "validation"}},
		{stage: "infer", signals: []string{"infer", "inference", "predict"}},
		{stage: "data", signals: []string{"dataset", "dataloader", "data loader"}},
		{stage: "setup", signals: []string{"install", "setup", "environment", "env var"}},
	}
	for _, candidate := range stageSignals {
		for _, signal := range candidate.signals {
			if strings.Contains(lower, signal) {
				return candidate.stage
			}
		}
	}
	return ""
}

func inferFrameworks(text string) []pack.DiagnosticFramework {
	frameworks := make([]pack.DiagnosticFramework, 0, 2)
	for _, name := range presentSignals(text, []string{"mindspore", "torch", "torch_npu", "tensorflow"}, 4) {
		frameworks = append(frameworks, pack.DiagnosticFramework{Name: name})
	}
	return frameworks
}

func inferAccelerator(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "ascend") || strings.Contains(lower, "npu") || strings.Contains(lower, "cann") || strings.Contains(lower, "acl") {
		return "ascend"
	}
	if strings.Contains(lower, "cuda") || strings.Contains(lower, "gpu") {
		return "gpu"
	}
	return ""
}

func presentSignals(text string, signals []string, limit int) []string {
	lower := strings.ToLower(text)
	out := make([]string, 0, limit)
	seen := make(map[string]struct{}, len(signals))
	for _, signal := range signals {
		if strings.Contains(lower, signal) {
			value := strings.TrimSpace(signal)
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// buildSkillTask constructs a task description that instructs the agent to load
// the appropriate diagnosis skill in the given mode (diagnose or fix).
func (a *Application) buildSkillTask(target issueCommandTarget, mode string) (string, error) {
	if target.HasIssue {
		issueCtx, err := a.buildIssueContext(target.IssueID)
		if err != nil {
			return "", fmt.Errorf("fetch issue failed: %w", err)
		}
		task := fmt.Sprintf("Load skill %s-agent in %s mode.\n\n%s", issueCtx.kind, mode, issueCtx.text)
		if target.Prompt != "" {
			task += "\n\nAdditional context: " + target.Prompt
		}
		return task, nil
	}

	return fmt.Sprintf(
		"Load the appropriate diagnosis skill (failure-agent, accuracy-agent, or performance-agent) in %s mode.\n\nUser problem: %s",
		mode, target.Prompt,
	), nil
}

type issueContext struct {
	kind string // failure, accuracy, performance
	text string // formatted issue details
}

func (a *Application) buildIssueContext(id int) (issueContext, error) {
	issue, err := a.issueService.GetIssue(id)
	if err != nil {
		return issueContext{}, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Issue: %s — %s\n", issue.Key, issue.Title)
	fmt.Fprintf(&b, "Kind: %s\n", issue.Kind)
	if issue.Summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", issue.Summary)
	}

	notes, err := a.issueService.ListNotes(id)
	if err == nil && len(notes) > 0 {
		b.WriteString("\nNotes:\n")
		for _, n := range notes {
			fmt.Fprintf(&b, "- [%s] %s\n", n.Author, n.Content)
		}
	}

	return issueContext{kind: string(issue.Kind), text: b.String()}, nil
}

func (a *Application) emitIssueDetail(id int, fromIndex bool) {
	issue, err := a.issueService.GetIssue(id)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("get issue failed: %v", err)}
		return
	}
	notes, err := a.issueService.ListNotes(id)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("list issue notes failed: %v", err)}
		return
	}
	acts, err := a.issueService.GetActivity(id)
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("list issue activity failed: %v", err)}
		return
	}
	a.EventCh <- model.Event{
		Type: model.IssueDetailOpen,
		IssueView: &model.IssueEventData{
			ID:        id,
			Issue:     issue,
			Notes:     notes,
			Activity:  acts,
			FromIndex: fromIndex,
		},
	}
}

func parseIssueReportInput(input string) (issuepkg.Kind, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("Usage: /report <failure|accuracy|performance> <title>")
	}
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("Usage: /report <failure|accuracy|performance> <title>")
	}
	kind, err := issuepkg.NormalizeKind(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("Usage: /report <failure|accuracy|performance> <title>")
	}
	title := strings.TrimSpace(strings.TrimPrefix(input, parts[0]))
	if title == "" {
		return "", "", fmt.Errorf("Usage: /report <failure|accuracy|performance> <title>")
	}
	return kind, title, nil
}

func parseIssueRef(ref string) (int, error) {
	ref = strings.TrimSpace(strings.ToUpper(ref))
	ref = strings.TrimPrefix(ref, "ISSUE-")
	return strconv.Atoi(ref)
}

func splitIssueNoteInput(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("Usage: /__issue_note <ISSUE-id> <content>")
	}
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("Usage: /__issue_note <ISSUE-id> <content>")
	}
	ref := parts[0]
	content := strings.TrimSpace(strings.TrimPrefix(input, ref))
	if content == "" {
		return "", "", fmt.Errorf("Usage: /__issue_note <ISSUE-id> <content>")
	}
	return ref, content, nil
}

type issueCommandTarget struct {
	HasIssue bool
	IssueID  int
	Prompt   string
}

func parseIssueCommandTarget(input string, command string) (issueCommandTarget, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return issueCommandTarget{}, fmt.Errorf("Usage: %s <problem text|ISSUE-id>", command)
	}

	parts := strings.Fields(trimmed)
	first := parts[0]
	if looksLikeIssueKey(first) {
		id, err := parseIssueRef(first)
		if err != nil {
			return issueCommandTarget{}, fmt.Errorf("invalid issue id")
		}
		return issueCommandTarget{
			HasIssue: true,
			IssueID:  id,
			Prompt:   strings.TrimSpace(strings.TrimPrefix(trimmed, first)),
		}, nil
	}

	return issueCommandTarget{Prompt: trimmed}, nil
}

func looksLikeIssueKey(token string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(token)), "ISSUE-")
}
