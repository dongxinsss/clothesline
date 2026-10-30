// Build-time only: match Wails' application icon resource ID 3.
package main

import (
	"fmt"
	"github.com/tc-hib/winres"
	"os"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func generate() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: winicon input.ico output_windows_amd64.syso")
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		return err
	}
	defer f.Close()
	icon, err := winres.LoadICO(f)
	if err != nil {
		return err
	}
	rs := winres.ResourceSet{}
	if err = rs.SetIcon(winres.ID(3), icon); err != nil {
		return err
	}
	out, err := os.Create(os.Args[2])
	if err != nil {
		return err
	}
	if err = rs.WriteObject(out, winres.ArchAMD64); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
