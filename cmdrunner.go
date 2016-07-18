package cmdrunner

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
	"syscall"
)

// CmdChannel represents the constant type of stdout or stderr.
type CmdChannel int

const (
	// CmdStdout is the constant representing stdout.
	CmdStdout CmdChannel = iota
	// CmdStderr is the constant representing stderr.
	CmdStderr
)

// OutputSink represents a function that will receive CmdOutput structs.
type OutputSink func(out *CmdOutput)

// CmdOutput is the combination of channel and text output.
// Empty lines from the output are not suppressed by this framework.
type CmdOutput struct {
	Channel CmdChannel ``
	Text    string
}

// CmdRunner wraps exec.Cmd to provide additional functionality.
type CmdRunner struct {
	cmd    *exec.Cmd
	output chan *CmdOutput
	wg     sync.WaitGroup
	sink   OutputSink
}

// NewCmdRunner creates a new CmdRunner, wrapping the provided exec.Cmd.
func NewCmdRunner(cmd *exec.Cmd) *CmdRunner {
	ch := make(chan *CmdOutput, 1)
	return &CmdRunner{cmd: cmd, output: ch}
}

func (r *CmdRunner) readPipe(readerFunc func() (io.ReadCloser, error), ch CmdChannel) error {

	reader, err := readerFunc()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(reader)

	r.wg.Add(1)
	go func() {
		for scanner.Scan() {
			r.output <- &CmdOutput{Channel: ch, Text: scanner.Text()}
		}
		r.wg.Done()
	}()

	return nil
}

// Start creates the stdout and stderr readers and sends output to the provided OutputSink.
func (r *CmdRunner) Start(outputSink OutputSink) error {
	r.sink = outputSink

	go func() {
		for cmdOut := range r.output {
			r.sink(cmdOut)
		}
	}()

	if err := r.readPipe(r.cmd.StdoutPipe, CmdStdout); err != nil {
		return err
	}
	if err := r.readPipe(r.cmd.StderrPipe, CmdStderr); err != nil {
		return err
	}

	return r.cmd.Start()

}

// WaitExit waits for the command to exit, returning the exit code.
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
