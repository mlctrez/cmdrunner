package cmdrunner

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
	"syscall"
)

type CmdChannel int

const (
	CmdStdout CmdChannel = iota
	CmdStderr
)

type OutputSink interface {
	HandleOutput(out *CmdOutput)
}

type CmdOutput struct {
	Channel CmdChannel``
	Text    string
}

type CmdRunner struct {
	cmd    *exec.Cmd
	output chan *CmdOutput
	wg     sync.WaitGroup
	sink   OutputSink
}

func NewCmdRunner(cmd *exec.Cmd) *CmdRunner {
	ch := make(chan *CmdOutput, 1)
	return &CmdRunner{cmd: cmd, output: ch}
}

func (r *CmdRunner) readPipe(pipeFunc func() (io.ReadCloser, error), ch CmdChannel) error {

	stdoutReader, err := pipeFunc()
	if err != nil {
		return err
	}

	stdoutScanner := bufio.NewScanner(stdoutReader)

	r.wg.Add(1)
	go func() {
		for stdoutScanner.Scan() {
			r.output <- &CmdOutput{Channel: ch, Text: stdoutScanner.Text()}
		}
		r.wg.Done()
	}()

	return nil
}

func (r *CmdRunner) Start(outputSink OutputSink) error {
	r.sink = outputSink

	go func() {
		for {
			cmdOut := <-r.output
			if cmdOut != nil {
				r.sink.HandleOutput(cmdOut)
			}
		}
	}()

	err := r.readPipe(r.cmd.StdoutPipe, CmdStdout)
	if err != nil {
		return err
	}
	err = r.readPipe(r.cmd.StderrPipe, CmdStderr)
	if err != nil {
		return err
	}

	return r.cmd.Start()

}

func (r *CmdRunner) WaitExit() int {

	r.wg.Wait()

	var waitStatus syscall.WaitStatus

	if err := r.cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
		}
	} else {
		waitStatus = r.cmd.ProcessState.Sys().(syscall.WaitStatus)
	}

	return waitStatus.ExitStatus()
}
