package pkg

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"
)

func RunCommand(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Debug("Execute:", "cmd", name+" "+strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error: %v\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
