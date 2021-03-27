package command

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mitchellh/cli"
)

// 工厂返回Command实例
// Factory is a function that returns a new instance of a CLI-sub command.
type Factory func(cli.Ui) (cli.Command, error)
// type CommandFactory func() (Command, error) 与cli包中的cli.Command的差别
// 传入的cli.Ui 可以用于对 Command 进行初始化

// Register adds a new CLI sub-command to the registry.
func Register(name string, fn Factory) {
	if registry == nil {
		registry = make(map[string]Factory)
	}

	if registry[name] != nil {
		panic(fmt.Errorf("Command %q is already registered", name))
	}
	registry[name] = fn
}

// 将Factory与cli的CommandFactory进行转化
// Map returns a realized mapping of available CLI commands in a format that
// the CLI class can consume. This should be called after all registration is
// complete.
func Map(ui cli.Ui) map[string]cli.CommandFactory {
	m := make(map[string]cli.CommandFactory)
	for name, fn := range registry {
		thisFn := fn
		m[name] = func() (cli.Command, error) {
			return thisFn(ui)
		}
	}
	return m
}

// 在init时初始化各个命令对应的map
// registry has an entry for each available CLI sub-command, indexed by sub
// command name. This should be populated at package init() time via Register().
var registry map[string]Factory

// 给每个command发送结束程序信号
// MakeShutdownCh returns a channel that can be used for shutdown notifications
// for commands. This channel will send a message for every interrupt or SIGTERM
// received.
func MakeShutdownCh() <-chan struct{} {
	resultCh := make(chan struct{})
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			<-signalCh
			resultCh <- struct{}{}
		}
	}()

	return resultCh
}
