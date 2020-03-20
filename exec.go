package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type datedline struct {
	date time.Time
	line []byte
}

type lifecycle interface {
	setup() *executor
	teardown() *executor
}

type executor struct {
	cmd *exec.Cmd
	out []datedline
	err []datedline
}

func newExecutor(cmd []string) *executor {
	return &executor{
		cmd: exec.Command(cmd[0], cmd[1:]...),
	}
}

func (e *executor) run() error {
	stdout, err := e.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot make stdout pipe: %w", err)
	}
	stderr, err := e.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot make stderr pipe: %w", err)
	}
	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("cannot start process: %w", err)
	}
	// TODO: limit or write to temp file
	consume := func(r io.Reader) ([]datedline, error) {
		var out []datedline
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			out = append(out, datedline{date: time.Now(), line: sc.Bytes()})
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return out, nil
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		var err error
		e.out, err = consume(stdout)
		errs[0] = err
		wg.Done()
	}()
	go func() {
		var err error
		e.err, err = consume(stderr)
		errs[1] = err
		wg.Done()
	}()
	err = e.cmd.Wait()
	wg.Wait()
	if err == nil {
		for _, e := range errs {
			if e != nil {
				err = e
			}
		}
	}
	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}
	return nil
}
