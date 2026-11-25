package lifecycle

type BuildPolicy struct {
	Build     bool
	SkipBuild bool
}

func (p BuildPolicy) Flag() string {
	if p.SkipBuild {
		return "--no-build"
	}
	if p.Build {
		return "--build"
	}
	return ""
}
