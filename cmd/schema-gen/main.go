// schema-gen generates internal/singboxconfig/schema.yaml from official
// sing-box documentation across release versions.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var (
		repoURL = flag.String("repo", "https://github.com/SagerNet/sing-box.git", "sing-box repository URL")
		repoDir = flag.String("repo-dir", "/tmp/sing-box-schema-repo", "local cache directory for the cloned repo")
		outPath = flag.String("out", "internal/singboxconfig/schema.yaml", "output schema path")
	)
	flag.Parse()

	absOut, err := filepath.Abs(*outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve output path: %v\n", err)
		os.Exit(1)
	}

	repo, err := OpenRepo(*repoDir, *repoURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo: %v\n", err)
		os.Exit(1)
	}
	if err := repo.FetchTags(); err != nil {
		fmt.Fprintf(os.Stderr, "fetch tags: %v\n", err)
		os.Exit(1)
	}

	tags, err := repo.ReleaseTags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "list tags: %v\n", err)
		os.Exit(1)
	}
	if len(tags) == 0 {
		fmt.Fprintf(os.Stderr, "no release tags found\n")
		os.Exit(1)
	}
	tags = OnePerMinor(tags)

	fmt.Printf("schema-gen: processing %d versions from %s to %s\n", len(tags), tags[0].Tag, tags[len(tags)-1].Tag)

	builder := NewSchemaBuilder(repo, tags)
	schema, err := builder.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build schema: %v\n", err)
		os.Exit(1)
	}

	if err := WriteYAML(schema, absOut); err != nil {
		fmt.Fprintf(os.Stderr, "write schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("schema-gen: wrote schema to %s (singbox_latest=%s)\n", absOut, schema.SingboxLatest)
}
