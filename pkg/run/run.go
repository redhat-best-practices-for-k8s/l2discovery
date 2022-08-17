package exec

import (
	"bytes"
	"os/exec"

	"github.com/sirupsen/logrus"
)

func LocalCommand(command string) (outStr, errStr string, err error) {
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return "", "", err
	}
	outStr, errStr = stdout.String(), stderr.String()
	logrus.Tracef("Command %s, STDERR: %s, STDOUT: %s", cmd.String(), errStr, outStr)
	return outStr, errStr, err
}
