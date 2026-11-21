package lifecycle

func BuildFingerprintInputs(compose, dockerfile, context []byte) string {
	return Fingerprint(compose, dockerfile, context)
}
