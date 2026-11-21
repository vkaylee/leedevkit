package lifecycle

type BuildPolicy struct {
	Build     bool
	SkipBuild bool
}

func (p BuildPolicy) Flag() string {
	if p.Build && !p.SkipBuild {
		return "--build"
	}
	return "--no-build"
}
