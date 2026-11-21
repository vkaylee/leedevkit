package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vkaylee/leedevkit/internal/runtime"
)

func main() {
	root := flag.String("root", ".", "release root")
	verify := flag.Bool("verify", false, "verify existing manifest")
	flag.Parse()
	var err error
	if *verify {
		var result runtime.ManifestResult
		result, err = runtime.VerifyManifest(*root)
		if err == nil && !result.Clean() {
			err = fmt.Errorf("manifest verification failed: modified=%d missing=%d extra=%d invalid=%d", len(result.Modified), len(result.Missing), len(result.Extra), len(result.Invalid))
		}
	} else {
		err = runtime.WriteManifest(*root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
