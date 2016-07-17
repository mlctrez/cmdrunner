package main

import (
	"fmt"
	"github.com/mlctrez/cmdrunner"
	"os/exec"
)

func main() {

	c := exec.Command("env")
	r := cmdrunner.NewCmdRunner(c)

	r.Start(func(out *cmdrunner.CmdOutput) {
		fmt.Println(out)
	})
	r.WaitExit()

}
