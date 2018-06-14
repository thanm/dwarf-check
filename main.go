//
// This program reads in DWARF info for a given load module (shared
// library or executable) and inspects it for problems/insanities,
// primarily abstract origin references that are incorrect.
// Can be run on object files as well, in theory.
//
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var verbflag = flag.Int("v", 0, "Verbose trace output level")
var iterflag = flag.Int("iters", 1, "Number of iterations")
var st int

func verb(vlevel int, s string, a ...interface{}) {
	if *verbflag >= vlevel {
		fmt.Printf(s, a...)
		fmt.Printf("\n")
	}
}

func warn(s string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, s, a...)
	fmt.Fprintf(os.Stderr, "\n")
	st = 1
}

func usage(msg string) {
	if len(msg) > 0 {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	fmt.Fprintf(os.Stderr, "usage: dwarf-check [flags] <ELF files>\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("dwarf-check: ")
	flag.Parse()
	verb(1, "in main")
	if flag.NArg() == 0 {
		usage("please supply one or more ELF files as command line arguments")
	}
	for _, arg := range flag.Args() {
		for i := 0; i < *iterflag; i++ {
			examineFile(arg)
		}
	}
	verb(1, "leaving main")
	os.Exit(st)
}
