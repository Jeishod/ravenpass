// Command androidstrings writes the Android string resources from a string source file, or standard input for "-".
package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/dortanes/ravenpass/apps/mobile/internal/resources"
)

func main() {
	source := flag.String("source", "-", "the string source, - for standard input")
	res := flag.String("res", "build/android/app/src/main/res", "the Android res directory")
	flag.Parse()
	var data []byte
	var err error
	if *source == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*source)
	}
	if err != nil {
		log.Fatal(err)
	}
	catalog, err := resources.Parse(data)
	if err != nil {
		log.Fatal(err)
	}
	for _, language := range catalog.Languages() {
		directory := filepath.Join(*res, resources.Directory(language))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "strings.xml"), catalog.Resources(language), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
