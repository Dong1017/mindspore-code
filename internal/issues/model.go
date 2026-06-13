package issues

type Kind string

const (
	KindFailure     Kind = "failure"
	KindAccuracy    Kind = "accuracy"
	KindPerformance Kind = "performance"
)
