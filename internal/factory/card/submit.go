package card

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultSubmissionsDir = "factory/submissions"

type SubmitOptions struct {
	OutputRoot string
	HomeDir    string
}

type ReviewBundle struct {
	CardID         string
	Path           string
	CardPath       string
	SummaryPath    string
	ValidationPath string
}

type ValidationResult struct {
	Schema          ValidationCheck `json:"schema"`
	Privacy         ValidationCheck `json:"privacy"`
	Draft           ValidationCheck `json:"draft"`
	PackReadiness   ValidationCheck `json:"pack_readiness"`
	NotReadyForPack bool            `json:"not_ready_for_pack"`
}

type ValidationCheck struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

func SubmitDraftCard(cardPath string, opts SubmitOptions) (*ReviewBundle, error) {
	resolved, err := resolveExplicitCardPath(cardPath)
	if err != nil {
		return nil, err
	}
	loaded, err := LoadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("load draft card: %w", err)
	}
	if err := SanitizeDraftCard(loaded, DraftOptions{HomeDir: opts.HomeDir}); err != nil {
		return nil, err
	}
	validation := BuildValidationResult(loaded)
	if !validation.Privacy.OK {
		return nil, fmt.Errorf("privacy validation failed: %s", validation.Privacy.Message)
	}
	if !validation.Draft.OK {
		return nil, fmt.Errorf("draft validation failed: %s", validation.Draft.Message)
	}

	root := opts.OutputRoot
	if strings.TrimSpace(root) == "" {
		root = DefaultSubmissionsDir
	}
	bundleDir := filepath.Join(root, loaded.ID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create submissions dir: %w", err)
	}
	if err := os.Mkdir(bundleDir, 0o700); err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("review bundle already exists: %s", bundleDir)
		}
		return nil, fmt.Errorf("create review bundle: %w", err)
	}
	bundle := &ReviewBundle{
		CardID:         loaded.ID,
		Path:           bundleDir,
		CardPath:       filepath.Join(bundleDir, "card.yaml"),
		SummaryPath:    filepath.Join(bundleDir, "summary.md"),
		ValidationPath: filepath.Join(bundleDir, "validation.json"),
	}
	if err := writeReviewBundleFiles(bundle, loaded, validation); err != nil {
		_ = os.RemoveAll(bundleDir)
		return nil, err
	}
	return bundle, nil
}

func BuildValidationResult(card *KnownIssueCard) ValidationResult {
	result := ValidationResult{}
	if card == nil {
		msg := "card is nil"
		result.Schema = ValidationCheck{OK: false, Message: msg}
		result.Privacy = ValidationCheck{OK: false, Message: msg}
		result.Draft = ValidationCheck{OK: false, Message: msg}
		result.PackReadiness = ValidationCheck{OK: false, Message: msg}
		result.NotReadyForPack = true
		return result
	}
	result.Schema = checkValidation(validateCommon(card))
	result.Privacy = checkValidation(ValidatePrivacy(card))
	result.Draft = checkValidation(ValidateDraft(card))
	packReady := checkValidation(ValidatePackEligible(card))
	result.PackReadiness = packReady
	result.NotReadyForPack = !packReady.OK
	return result
}

func RenderReviewSummary(card *KnownIssueCard, validation ValidationResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Factory Card Review: %s\n\n", safeSummaryLine(card.ID, 16))
	fmt.Fprintf(&b, "## Title\n%s\n\n", safeSummaryLine(card.Title, 40))
	fmt.Fprintf(&b, "## Problem\n- type: %s\n- stage: %s\n", safeSummaryLine(card.Problem.ProblemType, 8), safeSummaryLine(card.Problem.Stage, 8))
	writeSummaryList(&b, "symptoms", card.Problem.Symptoms, 6)
	b.WriteString("\n## Diagnosis\n")
	fmt.Fprintf(&b, "- explanation: %s\n", safeSummaryLine(card.Diagnosis.Explanation, 60))
	if strings.TrimSpace(card.Diagnosis.ScopeNote) != "" {
		fmt.Fprintf(&b, "- scope_note: %s\n", safeSummaryLine(card.Diagnosis.ScopeNote, 60))
	}
	if strings.TrimSpace(card.Diagnosis.RootCause) != "" {
		fmt.Fprintf(&b, "- root_cause: %s\n", safeSummaryLine(card.Diagnosis.RootCause, 60))
	}
	b.WriteString("\n## Fix\n")
	if strings.TrimSpace(card.Fix.Summary) == "" {
		b.WriteString("- summary: needs review\n")
	} else {
		fmt.Fprintf(&b, "- summary: %s\n", safeSummaryLine(card.Fix.Summary, 60))
	}
	b.WriteString("\n## Verification\n")
	writeSummaryList(&b, "checks", card.Verification.Checks, 4)
	b.WriteString("\n## Validation\n")
	fmt.Fprintf(&b, "- schema: %s\n", checkStatus(validation.Schema))
	fmt.Fprintf(&b, "- privacy: %s\n", checkStatus(validation.Privacy))
	fmt.Fprintf(&b, "- draft: %s\n", checkStatus(validation.Draft))
	fmt.Fprintf(&b, "- pack_readiness: %s\n", checkStatus(validation.PackReadiness))
	b.WriteString("\n## Reviewer notes\nDraft review is required before stable promotion. Do not approve without checking current evidence.\n")
	return b.String()
}

func resolveExplicitCardPath(cardPath string) (string, error) {
	cardPath = strings.TrimSpace(cardPath)
	if cardPath == "" {
		return "", fmt.Errorf("card path is required")
	}
	resolved, err := filepath.Abs(cardPath)
	if err != nil {
		return "", fmt.Errorf("resolve card path: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("card path does not exist: %s", cardPath)
		}
		return "", fmt.Errorf("stat card path: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("card path is a directory: %s", cardPath)
	}
	return resolved, nil
}

func writeReviewBundleFiles(bundle *ReviewBundle, card *KnownIssueCard, validation ValidationResult) error {
	cardData, err := yaml.Marshal(card)
	if err != nil {
		return fmt.Errorf("render sanitized card yaml: %w", err)
	}
	validationData, err := json.MarshalIndent(validation, "", "  ")
	if err != nil {
		return fmt.Errorf("render validation json: %w", err)
	}
	files := []struct {
		path string
		data []byte
	}{
		{bundle.CardPath, cardData},
		{bundle.SummaryPath, []byte(RenderReviewSummary(card, validation))},
		{bundle.ValidationPath, append(validationData, '\n')},
	}
	for _, file := range files {
		if err := os.WriteFile(file.path, file.data, 0o600); err != nil {
			return fmt.Errorf("write review bundle file: %w", err)
		}
	}
	return nil
}

func checkValidation(err error) ValidationCheck {
	if err == nil {
		return ValidationCheck{OK: true}
	}
	return ValidationCheck{OK: false, Message: err.Error()}
}

func writeSummaryList(b *strings.Builder, label string, values []string, limit int) {
	cleaned := boundedUnique(values, limit)
	if len(cleaned) == 0 {
		fmt.Fprintf(b, "- %s: needs review\n", label)
		return
	}
	fmt.Fprintf(b, "- %s:\n", label)
	for _, value := range cleaned {
		fmt.Fprintf(b, "  - %s\n", safeSummaryLine(value, 40))
	}
}

func safeSummaryLine(value string, maxWords int) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return trimWords(value, maxWords)
}

func checkStatus(check ValidationCheck) string {
	if check.OK {
		return "ok"
	}
	return "not ok - " + safeSummaryLine(check.Message, 24)
}
