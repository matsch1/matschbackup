package main

import (
	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
)

// ======================================================== CLI
var cli struct {
	Debug bool   `help:"Enable debug mode"`
	Full  Backup `cmd:"" help:"full backup"`
}

// ======================================================== Backup
type Backup struct {
	Srcdir string `help:"directory to backup"`
	Tgtdir string `help:"target directory for backup"`
}

func (fcmd Backup) Run(bkp *Backup) error {
	log.Info("Full backup running..." + bkp.Srcdir)
	return nil
}

// ======================================================== main
func main() {
	// ClearScreen()
	log.Info("Starting CLI")

	ctx := kong.Parse(&cli)

	err := ctx.Run(&cli)
	ctx.FatalIfErrorf(err)
}
