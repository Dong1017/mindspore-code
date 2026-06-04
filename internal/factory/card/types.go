package card

const (
	SchemaVersionKnownIssueV05 = "known_issue/v0.5"
	KindKnownIssue             = "known_issue"
)

const (
	ProblemTypeFailure     = "failure"
	ProblemTypeAccuracy    = "accuracy"
	ProblemTypePerformance = "performance"
	ProblemTypeUnknown     = "unknown"
)

const (
	StageSetup     = "setup"
	StageImport    = "import"
	StageTrain     = "train"
	StageEval      = "eval"
	StageInfer     = "infer"
	StageCompile   = "compile"
	StageData      = "data"
	StageGraphOpt  = "graph_opt"
	StageExecution = "execution"
	StageUnknown   = "unknown"
)

const (
	DomainMindSpore = "mindspore"
	DomainTorch     = "torch"
	DomainTorchNPU  = "torch_npu"
	DomainCANN      = "cann"
	DomainUnknown   = "unknown"
)

const (
	HardwareAscend  = "ascend"
	HardwareGPU     = "gpu"
	HardwareCPU     = "cpu"
	HardwareUnknown = "unknown"
)

const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
	SeverityUnknown  = "unknown"
)

const (
	FrameworkTorch     = "torch"
	FrameworkTorchNPU  = "torch_npu"
	FrameworkMindSpore = "mindspore"
	FrameworkUnknown   = "unknown"
)

const (
	ConfidenceBootstrap = "bootstrap"
	ConfidenceObserved  = "observed"
	ConfidenceVerified  = "verified"
)

const (
	LifecycleDraft      = "draft"
	LifecycleStable     = "stable"
	LifecycleDeprecated = "deprecated"
	LifecycleArchived   = "archived"
)

const (
	ReviewPending  = "pending"
	ReviewApproved = "approved"
	ReviewRejected = "rejected"
	ReviewUnknown  = "unknown"
)

type KnownIssueCard struct {
	SchemaVersion string     `yaml:"schema_version"`
	Kind          string     `yaml:"kind"`
	ID            string     `yaml:"id"`
	Title         string     `yaml:"title"`
	Tags          []string   `yaml:"tags"`
	Case          Case       `yaml:"case"`
	Match         Match      `yaml:"match"`
	Guidance      Guidance   `yaml:"guidance"`
	Provenance    Provenance `yaml:"provenance"`
	Governance    Governance `yaml:"governance"`
}

type Case struct {
	ProblemType     string      `yaml:"problem_type"`
	Stage           string      `yaml:"stage"`
	Domain          string      `yaml:"domain"`
	Hardware        string      `yaml:"hardware"`
	Severity        string      `yaml:"severity"`
	OccurrenceCount int         `yaml:"occurrence_count"`
	Environment     Environment `yaml:"environment"`
}

type Environment struct {
	Frameworks []Framework `yaml:"frameworks"`
	Runtime    Runtime     `yaml:"runtime"`
	Model      Model       `yaml:"model"`
	Affected   []string    `yaml:"affected"`
	FixedBy    []string    `yaml:"fixed_by"`
}

type Framework struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Branch  string `yaml:"branch"`
	Commit  string `yaml:"commit"`
}

type Runtime struct {
	CANNVersion   string `yaml:"cann_version"`
	PythonVersion string `yaml:"python_version"`
}

type Model struct {
	Pattern       string      `yaml:"pattern"`
	ExecutionMode string      `yaml:"execution_mode"`
	Optimization  string      `yaml:"optimization"`
	InputReuse    string      `yaml:"input_reuse"`
	DType         string      `yaml:"dtype"`
	InputShapes   InputShapes `yaml:"input_shapes"`
}

type InputShapes struct {
	OriginalReport string `yaml:"original_report"`
	RegressionNote string `yaml:"regression_note"`
}

type Match struct {
	Keywords []string `yaml:"keywords"`
	Regex    []string `yaml:"regex"`
}

type Guidance struct {
	Symptom              string   `yaml:"symptom"`
	TriggerSignals       []string `yaml:"trigger_signals"`
	RepresentativeErrors []string `yaml:"representative_errors"`
	Diagnosis            string   `yaml:"diagnosis"`
	DiagnosisDetails     []string `yaml:"diagnosis_details"`
	Actions              []string `yaml:"actions"`
	Fix                  string   `yaml:"fix"`
	WhyItWorks           []string `yaml:"why_it_works"`
	Verification         string   `yaml:"verification"`
	NonCauses            []string `yaml:"non_causes"`
}

type Provenance struct {
	References       []string `yaml:"references"`
	Notes            string   `yaml:"notes"`
	ExpectedBehavior []string `yaml:"expected_behavior"`
	RegressionTests  []string `yaml:"regression_tests"`
}

type Governance struct {
	Confidence   string `yaml:"confidence"`
	Lifecycle    string `yaml:"lifecycle"`
	ReviewStatus string `yaml:"review_status"`
	Rationale    string `yaml:"rationale"`
	UpdatedAt    string `yaml:"updated_at"`
}
