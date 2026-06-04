package card

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
)

var sensitiveDraftPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|password|passwd|secret)\s*[:=]`),
	regexp.MustCompile(`(?i)begin (rsa |openssh |ec )?private key`),
	regexp.MustCompile(`(?i)customer[_ -]?data`),
	regexp.MustCompile(`(?i)dataset sample`),
	regexp.MustCompile(`(?i)model weights?`),
}

func SanitizeDraftCard(card *KnownIssueCard, opts DraftOptions) error {
	if card == nil {
		return fmt.Errorf("card is nil")
	}
	home := strings.TrimSpace(opts.HomeDir)
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	sanitizeStruct(reflect.ValueOf(card).Elem(), home)
	if err := rejectUnsafeDraftText(card); err != nil {
		return err
	}
	return ValidatePrivacy(card)
}

func sanitizeStruct(value reflect.Value, home string) {
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		switch field.Kind() {
		case reflect.String:
			if field.CanSet() {
				field.SetString(sanitizeDraftText(field.String(), home))
			}
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				for j := 0; j < field.Len(); j++ {
					field.Index(j).SetString(sanitizeDraftText(field.Index(j).String(), home))
				}
			} else if field.Type().Elem().Kind() == reflect.Struct {
				for j := 0; j < field.Len(); j++ {
					sanitizeStruct(field.Index(j), home)
				}
			}
		case reflect.Struct:
			sanitizeStruct(field, home)
		}
	}
}

func sanitizeDraftText(value string, home string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if strings.TrimSpace(home) != "" {
		value = strings.ReplaceAll(value, home, "<HOME>")
	}
	return value
}

func rejectUnsafeDraftText(card *KnownIssueCard) error {
	for _, value := range collectPrivacyText(card) {
		if looksLikeRawDump(value) {
			return fmt.Errorf("privacy validation failed: raw logs, command output, env dumps, or large source excerpts must be summarized before writing a draft card")
		}
		for _, pattern := range sensitiveDraftPatterns {
			if pattern.MatchString(value) {
				return fmt.Errorf("privacy validation failed: sensitive content must be removed before writing a draft card")
			}
		}
	}
	return nil
}

func looksLikeRawDump(value string) bool {
	lines := strings.Split(value, "\n")
	if len(lines) > 12 {
		return true
	}
	if len(strings.Fields(value)) > 260 {
		return true
	}
	lower := strings.ToLower(value)
	if strings.Count(lower, "\n") >= 4 && (strings.Contains(lower, "traceback") || strings.Contains(lower, "error") || strings.Contains(lower, "exception")) {
		return true
	}
	if strings.Contains(lower, "env=") || strings.Contains(lower, "printenv") || strings.Contains(lower, "----- command output -----") {
		return true
	}
	return false
}
