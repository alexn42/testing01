// Command mediasearch searches an image, video, or PDF for text or a
// template image and prints a JSON report of whether (and where) it was
// found. See internal/cli for the implementation.
package main

import (
	"os"

	"mediasearch/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
