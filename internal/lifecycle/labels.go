package lifecycle

const BuildFingerprintLabel = "com.leedevkit.build-fingerprint"

func ImageLabelMatches(labels map[string]string, expected string) bool {
	return expected != "" && labels != nil && labels[BuildFingerprintLabel] == expected
}
