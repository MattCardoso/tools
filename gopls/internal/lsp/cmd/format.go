// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"

	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/gopls/internal/span"
	"golang.org/x/tools/internal/diff"
)

// format implements the format verb for gopls.
type format struct {
	Diff  bool `flag:"d,diff" help:"display diffs instead of rewriting files"`
	Write bool `flag:"w,write" help:"write result to (source) file instead of stdout"`
	List  bool `flag:"l,list" help:"list files whose formatting differs from gofmt's"`

	app *Application
}

func (c *format) Name() string      { return "format" }
func (c *format) Parent() string    { return c.app.Name() }
func (c *format) Usage() string     { return "[format-flags] <filerange>" }
func (c *format) ShortHelp() string { return "format the code according to the go standard" }
func (c *format) DetailedHelp(f *flag.FlagSet) {
	fmt.Fprint(f.Output(), `
The arguments supplied may be simple file names, or ranges within files.

Example: reformat this file:

	$ gopls format -w internal/lsp/cmd/check.go

format-flags:
`)
	printFlagDefaults(f)
}

// Run performs the check on the files specified by args and prints the
// results to stdout.
func (c *format) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		// no files, so no results
		return nil
	}
	// now we ready to kick things off
	conn, err := c.app.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.terminate(ctx)
	for _, arg := range args {
		spn := span.Parse(arg)
		file := conn.openFile(ctx, spn.URI())
		if file.err != nil {
			return file.err
		}
		filename := spn.URI().Filename()
		loc, err := file.mapper.SpanLocation(spn)
		if err != nil {
			return err
		}
		if loc.Range.Start != loc.Range.End {
			return fmt.Errorf("only full file formatting supported")
		}
		p := protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: loc.URI},
		}
		edits, err := conn.Formatting(ctx, &p)
		if err != nil {
			return fmt.Errorf("%v: %v", spn, err)
		}
		formatted, sedits, err := source.ApplyProtocolEdits(file.mapper, edits)
		if err != nil {
			return fmt.Errorf("%v: %v", spn, err)
		}
		printIt := true
		if c.List {
			printIt = false
			if len(edits) > 0 {
				fmt.Println(filename)
			}
		}
		if c.Write {
			printIt = false
			if len(edits) > 0 {
				ioutil.WriteFile(filename, []byte(formatted), 0644)
			}
		}
		if c.Diff {
			printIt = false
			unified, err := diff.ToUnified(filename+".orig", filename, string(file.mapper.Content), sedits)
			if err != nil {
				return err
			}
			fmt.Print(unified)
		}
		if printIt {
			fmt.Print(formatted)
		}
	}
	return nil
}
