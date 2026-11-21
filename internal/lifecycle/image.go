package lifecycle

type BuildMetadata struct {
	Expected string
	Actual   string
}

func (m BuildMetadata) Fresh() bool {
	return m.Expected != "" && m.Expected == m.Actual
}

func ShouldBuild(policy BuildPolicy, metadata BuildMetadata) bool {
	if policy.SkipBuild {
		return false
	}
	if policy.Build {
		return true
	}
	return !metadata.Fresh()
}
