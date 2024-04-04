package command

import (
	"bufio"
	"bytes"
	"errors"
	"os/exec"
	"strings"

	"github.com/forest33/tapir/business/entity"
)

type Executor struct {
	shellName string
	shellArgs []string
}

func New(cfg *entity.SystemConfig) (*Executor, error) {
	if len(cfg.Shell) == 0 {
		return nil, entity.ErrCommandShellUndefined
	}

	c := &Executor{}

	shell := strings.Split(cfg.Shell, " ")
	c.shellName = shell[0]
	if len(shell) > 1 {
		c.shellArgs = shell[1:]
	}

	return c, nil
}

func (c *Executor) Run(command string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	args := append(c.shellArgs, command)

	cmd := exec.Command(c.shellName, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", err
	} else if stderr.String() != "" {
		return stderr.String(), err
	}

	return stdout.String(), err
}

func (c *Executor) Start(command string) error {
	args := append(c.shellArgs, command)
	cmd := exec.Command(c.shellName, args...)
	return cmd.Start()
}

func (c *Executor) RunAndWaitResponse(command, successResponse, errorResponse string) error {
	args := append(c.shellArgs, command)
	cmd := exec.Command(c.shellName, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	stop := make(chan error)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Index(line, successResponse) != -1 {
				stop <- nil
				break
			} else if strings.Index(line, errorResponse) != -1 {
				stop <- errors.New("failed to execute")
				break
			}
		}
	}()

	return <-stop
}
