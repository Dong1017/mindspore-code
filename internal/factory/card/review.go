package card

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultApprovedCardsDir = "factory/cards"

type ReviewOptions struct {
	SubmissionsRoot string
}

type ApprovalOptions struct {
	SubmissionsRoot string
	CardsRoot       string
	Confidence      string
	Rationale       string
	Now             time.Time
}

type ReviewItem struct {
	Card       *KnownIssueCard
	Bundle     ReviewBundle
	Summary    string
	Validation ValidationResult
}

type ApprovalResult struct {
	CardID string
	Path   string
}

func RenderReviewItem(cardID string, opts ReviewOptions) (string, error) {
	item, err := LoadReviewItem(cardID, opts)
	if err != nil {
		return "", err
	}
	return RenderReviewItemView(item), nil
}

func LoadReviewItem(cardID string, opts ReviewOptions) (*ReviewItem, error) {
	bundle, err := resolveReviewBundle(cardID, opts.SubmissionsRoot)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(bundle.CardPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("review bundle missing card.yaml: %s", bundle.CardPath)
		}
		return nil, fmt.Errorf("stat review card: %w", err)
	}
	loaded, err := LoadFile(bundle.CardPath)
	if err != nil {
		return nil, fmt.Errorf("load review card: %w", err)
	}
	if loaded.ID != bundle.CardID {
		return nil, fmt.Errorf("review card id mismatch: bundle %s card %s", bundle.CardID, loaded.ID)
	}
	summaryData, err := os.ReadFile(bundle.SummaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("review bundle missing summary.md: %s", bundle.SummaryPath)
		}
		return nil, fmt.Errorf("read review summary: %w", err)
	}
	validationData, err := os.ReadFile(bundle.ValidationPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("review bundle missing validation.json: %s", bundle.ValidationPath)
		}
		return nil, fmt.Errorf("read review validation: %w", err)
	}
	var validation ValidationResult
	if err := json.Unmarshal(validationData, &validation); err != nil {
		return nil, fmt.Errorf("parse review validation.json: %w", err)
	}
	return &ReviewItem{Card: loaded, Bundle: *bundle, Summary: string(summaryData), Validation: validation}, nil
}

func ApproveReviewItem(cardID string, opts ApprovalOptions) (*ApprovalResult, error) {
	if opts.Confidence != ConfidenceObserved {
		return nil, fmt.Errorf("approval requires --confidence observed")
	}
	rationale := strings.TrimSpace(opts.Rationale)
	if rationale == "" {
		return nil, fmt.Errorf("approval requires non-empty --rationale")
	}
	item, err := LoadReviewItem(cardID, ReviewOptions{SubmissionsRoot: opts.SubmissionsRoot})
	if err != nil {
		return nil, err
	}
	if err := validateCommon(item.Card); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}
	if err := ValidatePrivacy(item.Card); err != nil {
		return nil, err
	}
	if err := ValidateDraft(item.Card); err != nil {
		return nil, fmt.Errorf("draft validation failed: %w", err)
	}
	approved := *item.Card
	approved.Governance.Lifecycle = LifecycleStable
	approved.Governance.ReviewStatus = ReviewApproved
	approved.Governance.Confidence = ConfidenceObserved
	approved.Governance.Rationale = rationale
	approved.Governance.UpdatedAt = approvalTime(opts.Now).UTC().Format(time.RFC3339)
	if err := ValidatePackEligible(&approved); err != nil {
		return nil, fmt.Errorf("pack readiness validation failed: %w", err)
	}
	cardsRoot := opts.CardsRoot
	if strings.TrimSpace(cardsRoot) == "" {
		cardsRoot = DefaultApprovedCardsDir
	}
	if err := os.MkdirAll(cardsRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create approved cards dir: %w", err)
	}
	path := filepath.Join(cardsRoot, item.Bundle.CardID+".yaml")
	data, err := yaml.Marshal(&approved)
	if err != nil {
		return nil, fmt.Errorf("render approved card yaml: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("approved card already exists: %s", path)
		}
		return nil, fmt.Errorf("create approved card: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("write approved card: %w", err)
	}
	return &ApprovalResult{CardID: approved.ID, Path: path}, nil
}

func RenderReviewItemView(item *ReviewItem) string {
	var b strings.Builder
	card := item.Card
	fmt.Fprintf(&b, "factory card review: %s\n", safeSummaryLine(item.Bundle.CardID, 16))
	fmt.Fprintf(&b, "card_id: %s\n", safeSummaryLine(item.Bundle.CardID, 16))
	fmt.Fprintf(&b, "title: %s\n", safeSummaryLine(card.Title, 40))
	fmt.Fprintf(&b, "schema_version: %s\n", safeSummaryLine(card.SchemaVersion, 8))
	fmt.Fprintf(&b, "case.problem_type: %s\n", safeSummaryLine(card.Case.ProblemType, 8))
	fmt.Fprintf(&b, "case.stage: %s\n", safeSummaryLine(card.Case.Stage, 8))
	fmt.Fprintf(&b, "case.domain: %s\n", safeSummaryLine(card.Case.Domain, 8))
	fmt.Fprintf(&b, "case.hardware: %s\n", safeSummaryLine(card.Case.Hardware, 8))
	fmt.Fprintf(&b, "governance.lifecycle: %s\n", safeSummaryLine(card.Governance.Lifecycle, 8))
	fmt.Fprintf(&b, "governance.review_status: %s\n", safeSummaryLine(card.Governance.ReviewStatus, 8))
	fmt.Fprintf(&b, "governance.confidence: %s\n", safeSummaryLine(card.Governance.Confidence, 8))
	b.WriteString("validation:\n")
	fmt.Fprintf(&b, "  schema: %s\n", checkStatus(item.Validation.Schema))
	fmt.Fprintf(&b, "  privacy: %s\n", checkStatus(item.Validation.Privacy))
	fmt.Fprintf(&b, "  draft: %s\n", checkStatus(item.Validation.Draft))
	fmt.Fprintf(&b, "  pack_readiness: %s\n", checkStatus(item.Validation.PackReadiness))
	fmt.Fprintf(&b, "  not_ready_for_pack: %t\n", item.Validation.NotReadyForPack)
	b.WriteString("summary_excerpt:\n")
	for _, line := range boundedSummaryExcerpt(item.Summary, 12) {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	b.WriteString("reviewer_checklist:\n")
	b.WriteString("  - Confirm evidence matches the current failure.\n")
	b.WriteString("  - Confirm guidance is safe and reproducible.\n")
	b.WriteString("  - Confirm pack readiness before manual approval.\n")
	b.WriteString("note: Manual review required. Review mode does not approve, promote, build, sync, or upload the card.")
	return b.String()
}

func resolveReviewBundle(cardID, root string) (*ReviewBundle, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return nil, fmt.Errorf("card id is required")
	}
	if strings.ContainsAny(cardID, `/\\`) || cardID == "." || cardID == ".." {
		return nil, fmt.Errorf("invalid card id: %s", cardID)
	}
	if strings.TrimSpace(root) == "" {
		root = DefaultSubmissionsDir
	}
	bundleDir := filepath.Join(root, cardID)
	info, err := os.Stat(bundleDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("review submission does not exist: %s", cardID)
		}
		return nil, fmt.Errorf("stat review submission: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("review submission is not a directory: %s", cardID)
	}
	return &ReviewBundle{CardID: cardID, Path: bundleDir, CardPath: filepath.Join(bundleDir, "card.yaml"), SummaryPath: filepath.Join(bundleDir, "summary.md"), ValidationPath: filepath.Join(bundleDir, "validation.json")}, nil
}

func boundedSummaryExcerpt(summary string, limit int) []string {
	lines := strings.Split(strings.ReplaceAll(summary, "\r\n", "\n"), "\n")
	out := make([]string, 0, limit)
	for _, line := range lines {
		line = safeSummaryLine(line, 28)
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
		if len(out) == limit {
			break
		}
	}
	if len(out) == 0 {
		return []string{"(empty)"}
	}
	return out
}

func approvalTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
