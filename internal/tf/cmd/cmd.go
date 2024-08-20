package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

type Cmd struct{}

type CmdArgs struct {
	Dir  string
	Run  string
	Args []string
}

func (c *Cmd) Execute(opts CmdArgs) (string, error) {
	cmd := exec.Command(opts.Run, opts.Args...)
	cmd.Dir = opts.Dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		combinedOutput := stdout.String() + "\n" + stderr.String()
		return combinedOutput, err
	}

	if stderr.Len() != 0 {
		return stdout.String(), errors.New(stderr.String())
	}

	return stdout.String(), nil
}

func (c *Cmd) ExecuteWithErr(opts CmdArgs) (string, string, error) {
	cmd := exec.Command(opts.Run, opts.Args...)
	cmd.Dir = opts.Dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", "", err
	}

	if stderr.Len() != 0 {
		return stdout.String(), stderr.String(), errors.New(stderr.String())
	}

	return stdout.String(), stderr.String(), nil
}

func (c *Cmd) ExecuteWithStream(opts CmdArgs) error {
	cmd := exec.Command(opts.Run, opts.Args...)
	cmd.Dir = opts.Dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
