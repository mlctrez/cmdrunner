package main

import (
	"github.com/mlctrez/cmdrunner"
	"os/exec"
	"context"
	"log"
	"time"
	"os"
	"fmt"
)

func exampleOne() {
	lc := buildLogContainer()
	// no cancel, blocks until command exits
	c := exec.Command("bash", "example/sleeper.sh", "2", "0")
	r := cmdrunner.NewCmdRunner(c).WithDebugLogger(lc.loggerCmd)
	r.Start(buildSink(lc.loggerMain))
	lc.loggerMain.Println("EXCODE", r.WaitExit())
	lc.loggerMain.Println("exampleOne exiting")

}

func exampleTwo(cancelSeconds time.Duration, sleeperSleepsFor, sleeperExitCode string) {
	lc := buildLogContainer()

	// this could be WithTimeout or WithDeadline but we want logging
	// around the time of cancellation below

	ctx, cancelFunc := context.WithCancel(context.Background())
	c := exec.Command("bash", "example/sleeper.sh", sleeperSleepsFor, sleeperExitCode)
	r := cmdrunner.NewCmdRunner(c).WithContext(ctx).WithDebugLogger(lc.loggerCmd)
	err := r.Start(buildSink(lc.loggerMain))
	if err != nil {
		panic(err)
	}

	go func() {
		lc.loggerMain.Println("EXCODE", r.WaitExit())
	}()

	time.Sleep(cancelSeconds)
	lc.loggerMain.Println("cancelling")
	cancelFunc()
	lc.loggerMain.Println("cancelling complete")
	time.Sleep(2 * time.Second)
	lc.loggerMain.Println("exampleTwo exiting")

}

func main() {

	exampleOne()
	fmt.Println()
	exampleTwo(2*time.Second, "5", "0")
	fmt.Println()
	exampleTwo(6*time.Second, "5", "20")

}

func buildSink(logger *log.Logger) cmdrunner.OutputSink {
	return func(out *cmdrunner.CmdOutput) {
		if out.Channel == cmdrunner.CmdStdout {
			logger.Println("STDOUT", out.Text)
		} else {
			logger.Println("STDERR", out.Text)
		}
	}
}

type LogContainer struct {
	loggerMain *log.Logger
	loggerCmd  *log.Logger
}

func buildLogContainer() *LogContainer {
	logFlags := log.LstdFlags | log.Lmicroseconds
	lc := &LogContainer{
		loggerMain: log.New(os.Stdout, "", logFlags),
		loggerCmd: log.New(os.Stderr, "", logFlags),
	}
	return lc
}
