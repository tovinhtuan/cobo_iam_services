package conflict

// Detector evaluates one rule against a read-only snapshot.
type Detector interface {
	Code() string
	Detect(snapshot *ConfigurationSnapshot) []Result
}
