package pack

import "strings"

type DiagnosticContext struct {
	RunID       string
	Command     string
	UserInput   string
	Problem     DiagnosticProblem
	Signals     DiagnosticSignals
	Environment DiagnosticEnvironment
}

type DiagnosticProblem struct {
	InferredType  string
	InferredStage string
}

type DiagnosticSignals struct {
	MainError       string
	TracebackTail   string
	LogTail         string
	Keywords        []string
	StackKeywords   []string
	NegativeSignals []string
}

type DiagnosticEnvironment struct {
	Hardware   DiagnosticHardware
	Runtime    DiagnosticRuntime
	Frameworks []DiagnosticFramework
}

type DiagnosticHardware struct {
	Accelerator string
	Chip        []string
}

type DiagnosticRuntime struct {
	CANNVersion   string
	DriverVersion string
	PythonVersion string
}

type DiagnosticFramework struct {
	Name    string
	Version string
}

type Fingerprint struct {
	Intent      string
	Stage       string
	ProblemType string
	Environment FingerprintEnvironment
	Signals     FingerprintSignals
}

type FingerprintEnvironment struct {
	Hardware   FingerprintHardware
	Runtime    FingerprintRuntime
	Frameworks []FingerprintFramework
}

type FingerprintHardware struct {
	Accelerator string
	Chip        []string
}

type FingerprintRuntime struct {
	CANNVersion   string
	DriverVersion string
	PythonVersion string
}

type FingerprintFramework struct {
	Name    string
	Version string
}

type FingerprintSignals struct {
	UserInput       string
	MainError       string
	TracebackTail   string
	LogTail         string
	Keywords        []string
	StackKeywords   []string
	NegativeSignals []string
}

func (ctx DiagnosticContext) ToFingerprint() Fingerprint {
	intent := strings.TrimPrefix(strings.TrimSpace(ctx.Command), "/")
	if intent == "" {
		intent = "diagnose"
	}
	return Fingerprint{
		Intent:      normalizeUnknown(intent),
		Stage:       normalizeUnknown(ctx.Problem.InferredStage),
		ProblemType: normalizeUnknown(ctx.Problem.InferredType),
		Environment: FingerprintEnvironment{
			Hardware: FingerprintHardware{
				Accelerator: normalizeUnknown(ctx.Environment.Hardware.Accelerator),
				Chip:        normalizeStringSlice(ctx.Environment.Hardware.Chip),
			},
			Runtime: FingerprintRuntime{
				CANNVersion:   strings.TrimSpace(ctx.Environment.Runtime.CANNVersion),
				DriverVersion: strings.TrimSpace(ctx.Environment.Runtime.DriverVersion),
				PythonVersion: strings.TrimSpace(ctx.Environment.Runtime.PythonVersion),
			},
			Frameworks: toFingerprintFrameworks(ctx.Environment.Frameworks),
		},
		Signals: FingerprintSignals{
			UserInput:       strings.TrimSpace(ctx.UserInput),
			MainError:       strings.TrimSpace(ctx.Signals.MainError),
			TracebackTail:   strings.TrimSpace(ctx.Signals.TracebackTail),
			LogTail:         strings.TrimSpace(ctx.Signals.LogTail),
			Keywords:        normalizeStringSlice(ctx.Signals.Keywords),
			StackKeywords:   normalizeStringSlice(ctx.Signals.StackKeywords),
			NegativeSignals: normalizeStringSlice(ctx.Signals.NegativeSignals),
		},
	}
}

func toFingerprintFrameworks(frameworks []DiagnosticFramework) []FingerprintFramework {
	out := make([]FingerprintFramework, 0, len(frameworks))
	for _, framework := range frameworks {
		name := normalizeUnknown(framework.Name)
		version := strings.TrimSpace(framework.Version)
		if name == "unknown" && version == "" {
			continue
		}
		out = append(out, FingerprintFramework{Name: name, Version: version})
	}
	return out
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeUnknown(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	return value
}
