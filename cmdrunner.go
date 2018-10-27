package cmdrunner

import (
	"bufio"
	"context"
	"io"
	"log"
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
	cmd          *exec.Cmd
	output       chan *CmdOutput
	wg           sync.WaitGroup
	sink         OutputSink
	ctx          context.Context
	debugLogger  *log.Logger
	cancelSignal syscall.Signal
}

// NewCmdRunner creates a new CmdRunner, wrapping the provided exec.Cmd.
func NewCmdRunner(cmd *exec.Cmd) *CmdRunner {
	ch := make(chan *CmdOutput, 100)
	return &CmdRunner{cmd: cmd, output: ch, cancelSignal: syscall.SIGTERM}
}

// WithContext adds a context to the command runner
func (r *CmdRunner) WithContext(ctx context.Context) *CmdRunner {
	if r.ctx != nil {
		r.ctx = ctx
	}
	return r
}

// WithCancelSignal sets the signal to pass to the process when the context is cancelled
func (r *CmdRunner) WithCancelSignal(signal syscall.Signal) *CmdRunner {
	r.cancelSignal = signal
	return r
}

// WithDebugLogger enables debugging messages.
// Logging goes to the provided logger.
func (r *CmdRunner) WithDebugLogger(logger *log.Logger) *CmdRunner {
	r.debugLogger = logger
	return r
}

func (r *CmdRunner) readPipe(readerFunc func() (io.ReadCloser, error), ch CmdChannel) error {

	dbgPrefix := "readPipe(stdout)"
	if CmdStderr == ch {
		dbgPrefix = "readPipe(stderr)"
	}

	reader, err := readerFunc()
	if err != nil {
		return err
	}

	ctx, cancelContext := r.withCancelContext()

	scannerChan := make(chan *CmdOutput, 100)
	r.wg.Add(1)
	go func() {
		defer r.debug(dbgPrefix + " exit scanner goroutine")
		defer r.wg.Done()
		defer cancelContext()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			scannerChan <- &CmdOutput{Channel: ch, Text: scanner.Text()}
		}
	}()

	r.wg.Add(1)
	go func() {
		defer r.debug(dbgPrefix + " exit selector goroutine")
		defer r.wg.Done()
		defer cancelContext()
		for {
			select {
			case <-ctx.Done():
				rce := reader.Close()
				r.debug(dbgPrefix+" reader.Close()", rce)
				return
			case co := <-scannerChan:
				r.output <- co
			}
		}
	}()

	return nil
}

func (r *CmdRunner) withCancelContext() (ctx context.Context, cancel context.CancelFunc) {
	if r.ctx != nil {
		return context.WithCancel(r.ctx)
	}
	return context.WithCancel(context.Background())
}

func (r *CmdRunner) sinkOutput() {
	r.debug("sinkOutput entry")
	defer r.debug("sinkOutput exit")

	ctx, cancelFunc := r.withCancelContext()
	defer cancelFunc()
	for {
		select {
		case co := <-r.output:
			r.debug("sinkOutput() <-r.output")
			r.sink(co)
		case <-ctx.Done():
			r.debug("sinkOutput() <-ctx.Done")
			return
		}
	}
}

func (r *CmdRunner) debug(in ...interface{}) {
	if r.debugLogger != nil {
		var out []interface{}
		out = append(out, "CmdRunner")
		out = append(out, in...)
		r.debugLogger.Println(out...)
	}
}

// Start creates the stdout and stderr readers and sends output to the provided OutputSink.
func (r *CmdRunner) Start(outputSink OutputSink) error {
	r.sink = outputSink

	go r.sinkOutput()

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

	ctx, cancel := r.withCancelContext()
	defer cancel()

	go func() {
		<-ctx.Done()
		if r.cmd != nil && r.cmd.Process != nil {
			r.debug("sending", r.cancelSignal)
			r.cmd.Process.Signal(r.cancelSignal)
		}
	}()

	r.wg.Wait()

	// look first for an exit code

	if err := r.cmd.Wait(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if waitStatus, ok := (ee.Sys()).(syscall.WaitStatus); ok {
				r.debug("returning exit status from error")
				return waitStatus.ExitStatus()
			}
			r.debug("unhandled system dependent process state")
		} else {
			r.debug("unhandled error type")
		}
		// non nil error deserves a non zero exit code
		return 1
	}

	procState := r.cmd.ProcessState.Sys()
	if procState != nil {
		if waitStatus, ok := procState.(syscall.WaitStatus); ok {
			r.debug("returning exit status from process state")
			return waitStatus.ExitStatus()
		}
		r.debug("unhandled system dependent process state")
	} else {
		r.debug("unable to get process state")
	}
	return 0
}
