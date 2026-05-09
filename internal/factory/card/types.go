package card

const (
	KindKnownIssue = "known_issue"
)

const (
	ProblemTypeFailure     = "failure"
	ProblemTypeAccuracy    = "accuracy"
	ProblemTypePerformance = "performance"
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
)

type KnownIssueCard struct {
	ID            string        `yaml:"id"`
	Kind          string        `yaml:"kind"`
	Title         string        `yaml:"title"`
	Problem       Problem       `yaml:"problem"`
	Environment   Environment   `yaml:"environment"`
	Applicability Applicability `yaml:"applicability"`
	Match         Match         `yaml:"match"`
	Diagnosis     Diagnosis     `yaml:"diagnosis"`
	Fix           Fix           `yaml:"fix"`
	Verification  Verification  `yaml:"verification"`
	Provenance    Provenance    `yaml:"provenance"`
	Confidence    Confidence    `yaml:"confidence"`
	Lifecycle     Lifecycle     `yaml:"lifecycle"`
	Review        Review        `yaml:"review"`
	Tags          []string      `yaml:"tags"`
}

type Problem struct {
	ProblemType string   `yaml:"problem_type"`
	Stage       string   `yaml:"stage"`
	Symptoms    []string `yaml:"symptoms"`
}

type Environment struct {
	Platform   Platform    `yaml:"platform"`
	Hardware   Hardware    `yaml:"hardware"`
	Runtime    Runtime     `yaml:"runtime"`
	Frameworks []Framework `yaml:"frameworks"`
}

type Platform struct {
	OS        OS     `yaml:"os"`
	Arch      string `yaml:"arch"`
	Container string `yaml:"container"`
}

type OS struct {
	Family  string `yaml:"family"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Hardware struct {
	Accelerator string   `yaml:"accelerator"`
	Chip        []string `yaml:"chip"`
}

type Runtime struct {
	CANNVersion   string `yaml:"cann_version"`
	DriverVersion string `yaml:"driver_version"`
	PythonVersion string `yaml:"python_version"`
}

type Framework struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Applicability struct {
	AffectedVersions []string `yaml:"affected_versions"`
	AffectedBranches []string `yaml:"affected_branches"`
	AffectedCommits  []string `yaml:"affected_commits"`
	IntroducedBy     []string `yaml:"introduced_by"`
	FixedBy          []string `yaml:"fixed_by"`
}

type Match struct {
	Keywords         []string `yaml:"keywords"`
	Regex            []string `yaml:"regex"`
	StackKeywords    []string `yaml:"stack_keywords"`
	NegativePatterns []string `yaml:"negative_patterns"`
}

type Diagnosis struct {
	RootCause           string   `yaml:"root_cause"`
	Explanation         string   `yaml:"explanation"`
	ScopeNote           string   `yaml:"scope_note"`
	SuggestedNextChecks []string `yaml:"suggested_next_checks"`
	MissingEvidence     []string `yaml:"missing_evidence"`
	ConflictingSignals  []string `yaml:"conflicting_signals"`
}

type Fix struct {
	Summary    string   `yaml:"summary"`
	Steps      []string `yaml:"steps"`
	Template   string   `yaml:"template"`
	WhyItWorks string   `yaml:"why_it_works"`
}

type Verification struct {
	Checks          []string `yaml:"checks"`
	Commands        []string `yaml:"commands"`
	ExpectedResult  string   `yaml:"expected_result"`
	RegressionTests []string `yaml:"regression_tests"`
}

type Provenance struct {
	References []string `yaml:"references"`
	Notes      string   `yaml:"notes"`
}

type Confidence struct {
	Level     string `yaml:"level"`
	Rationale string `yaml:"rationale"`
}

type Lifecycle struct {
	State      string   `yaml:"state"`
	Reason     string   `yaml:"reason"`
	ReplacedBy []string `yaml:"replaced_by"`
	UpdatedAt  string   `yaml:"updated_at"`
}

type Review struct {
	Status        string `yaml:"status"`
	ReviewedBy    string `yaml:"reviewed_by"`
	ReviewerNotes string `yaml:"reviewer_notes"`
}
