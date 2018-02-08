package cmdrunner

// generate an interface for CmdRunner

//go:generate ifacemaker -f ../cmdrunner.go -s CmdRunner -p cmdrunner -i CmdRunnerImpl -o iface.go -r cmdrunner -a github.com/mlctrez/cmdrunner
