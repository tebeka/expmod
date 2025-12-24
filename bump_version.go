//go:build ignore

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
)

func main() {
	var version, commit string
	flag.StringVar(&version, "version", "", "version string")
	flag.StringVar(&commit, "commit", "", "commit hash")
	flag.Parse()

	if version == "" || commit == "" {
		fmt.Fprintf(os.Stderr, "error: both -version and -commit are required\n")
		os.Exit(1)
	}

	versionRe := regexp.MustCompile(`(\s*version\s*=\s*)"[^"]*"`)
	commitRe := regexp.MustCompile(`(\s*commit\s*=\s*)"[^"]*"`)

	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		line := s.Text()

		// Replace version line
		if versionRe.MatchString(line) {
			line = versionRe.ReplaceAllString(line, fmt.Sprintf(`${1}"%s"`, version))
		}

		// Replace commit line
		if commitRe.MatchString(line) {
			line = commitRe.ReplaceAllString(line, fmt.Sprintf(`${1}"%s"`, commit))
		}

		fmt.Println(line)
	}

	if err := s.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error: scan - %v\n", err)
		os.Exit(1)
	}
}
