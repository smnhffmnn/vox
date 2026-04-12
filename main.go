package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version":
			fmt.Println("vox " + version)
			os.Exit(0)
		case "--help", "-help", "-h":
			printUsage()
			os.Exit(0)
		case "transcribe":
			os.Exit(runTranscribe(os.Args[2:]))
		}
	}

	runDesktop()
}
