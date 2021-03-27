package agent

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-checkpoint"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/service_os"
)

// agent命令初始化，对版本号等进行赋值
func New(ui cli.Ui, revision, version, versionPre, versionHuman string, shutdownCh <-chan struct{}) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

	c := &cmd{
		UI:                ui,
		revision:          revision,
		version:           version,
		versionPrerelease: versionPre,
		versionHuman:      versionHuman,
		shutdownCh:        shutdownCh,
		flags:             flag.NewFlagSet("", flag.ContinueOnError),
	}
	config.AddFlags(c.flags, &c.configLoadOpts)
	c.help = flags.Usage(help, c.flags)
	return c
}

// AgentCommand is a Command implementation that runs a Consul agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type cmd struct {
	UI                cli.Ui
	flags             *flag.FlagSet
	http              *flags.HTTPFlags
	help              string
	revision          string
	version           string
	versionPrerelease string
	versionHuman      string
	shutdownCh        <-chan struct{}
	configLoadOpts    config.LoadOpts
	logger            hclog.InterceptLogger
}

func (c *cmd) Run(args []string) int {
	code := c.run(args) // agent命令支持子命令，将子命令传入command
	if c.logger != nil {
		c.logger.Info("Exit code", "code", code)
	}
	return code
}

// checkpointResults is used to handler periodic results from our update checker
// 处理Check的相应结果
func (c *cmd) checkpointResults(results *checkpoint.CheckResponse, err error) {
	if err != nil {
		c.logger.Error("Failed to check for updates", "error", err)
		return
	}
	if results.Outdated {
		c.logger.Info("Newer Consul version available", "new_version", results.CurrentVersion, "current_version", c.version)
	}
	for _, alert := range results.Alerts {
		switch alert.Level {
		case "info":
			c.logger.Info("Bulletin", "alert_level", alert.Level, "alert_message", alert.Message, "alert_URL", alert.URL)
		default:
			c.logger.Error("Bulletin", "alert_level", alert.Level, "alert_message", alert.Message, "alert_URL", alert.URL)
		}
	}
}

// 启动的时候请求远端对相关的信息进行检查（版本号等）
func (c *cmd) startupUpdateCheck(config *config.RuntimeConfig) {
	version := config.Version
	if config.VersionPrerelease != "" {
		version += fmt.Sprintf("-%s", config.VersionPrerelease)
	}
	updateParams := &checkpoint.CheckParams{
		Product: "consul",
		Version: version,
	}
	if !config.DisableAnonymousSignature { // 决定是否在参数中发送SignatureFile
		updateParams.SignatureFile = filepath.Join(config.DataDir, "checkpoint-signature")
	}

	// Schedule a periodic check with expected interval of 24 hours
	checkpoint.CheckInterval(updateParams, 24*time.Hour, c.checkpointResults)

	// Do an immediate check within the next 30 seconds
	go func() {
		time.Sleep(lib.RandomStagger(30 * time.Second))
		c.checkpointResults(checkpoint.Check(updateParams))
	}()
}

// LAN局域网（local area network）
// startupJoin is invoked to handle any joins specified to take place at start time
func (c *cmd) startupJoin(agent *agent.Agent, cfg *config.RuntimeConfig) error {
	if len(cfg.StartJoinAddrsLAN) == 0 {
		return nil
	}

	c.UI.Output("Joining cluster...")
	n, err := agent.JoinLAN(cfg.StartJoinAddrsLAN)
	if err != nil {
		return err
	}

	c.logger.Info("Join completed. Initial agents synced with", "agent_count", n)
	return nil
}

// WAN广域网（wide area network）
// startupJoinWan is invoked to handle any joins -wan specified to take place at start time
func (c *cmd) startupJoinWan(agent *agent.Agent, cfg *config.RuntimeConfig) error {
	if len(cfg.StartJoinAddrsWAN) == 0 {
		return nil
	}

	c.UI.Output("Joining -wan cluster...")
	n, err := agent.JoinWAN(cfg.StartJoinAddrsWAN)
	if err != nil {
		return err
	}

	c.logger.Info("Join -wan completed. Initial agents synced with", "agent_count", n)
	return nil
}
// consul agent 启动方法
func (c *cmd) run(args []string) int {
	// 对
	if err := c.flags.Parse(args); err != nil {
		if !strings.Contains(err.Error(), "help requested") {
			c.UI.Error(fmt.Sprintf("error parsing flags: %v", err))
		}
		return 1
	}
	// 对agent的子命令进行解析
	if len(c.flags.Args()) > 0 {
		c.UI.Error(fmt.Sprintf("Unexpected extra arguments: %v", c.flags.Args()))
		return 1
	}

	logGate := &logging.GatedWriter{Writer: &cli.UiWriter{Ui: c.UI}}
	loader := func(source config.Source) (config.LoadResult, error) {
		c.configLoadOpts.DefaultConfig = source
		return config.Load(c.configLoadOpts)
	}
	bd, err := agent.NewBaseDeps(loader, logGate)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.logger = bd.Logger
	agent, err := agent.New(bd)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	config := bd.RuntimeConfig

	// Setup gate to check if we should output CLI information
	cli := GatedUi{
		JSONoutput: config.Logging.LogJSON,
		ui:         c.UI,
	}

	// Create the agent
	cli.output("Starting Consul agent...")

	segment := config.SegmentName
	if config.ServerMode {
		segment = "<all>"
	}
	// 输出相关信息
	cli.info(fmt.Sprintf("       Version: '%s'", c.versionHuman))
	cli.info(fmt.Sprintf("       Node ID: '%s'", config.NodeID))
	cli.info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	cli.info(fmt.Sprintf("    Datacenter: '%s' (Segment: '%s')", config.Datacenter, segment))
	cli.info(fmt.Sprintf("        Server: %v (Bootstrap: %v)", config.ServerMode, config.Bootstrap))
	cli.info(fmt.Sprintf("   Client Addr: %v (HTTP: %d, HTTPS: %d, gRPC: %d, DNS: %d)", config.ClientAddrs,
		config.HTTPPort, config.HTTPSPort, config.GRPCPort, config.DNSPort))
	cli.info(fmt.Sprintf("  Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddrLAN,
		config.SerfPortLAN, config.SerfPortWAN))
	cli.info(fmt.Sprintf("       Encrypt: Gossip: %v, TLS-Outgoing: %v, TLS-Incoming: %v, Auto-Encrypt-TLS: %t",
		config.EncryptKey != "", config.VerifyOutgoing, config.VerifyIncoming, config.AutoEncryptTLS || config.AutoEncryptAllowTLS))
	// Enable log streaming
	cli.output("")
	cli.output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// wait for signal
	signalCh := make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	ctx, cancel := context.WithCancel(context.Background())

	go func() { // 启动守护协程，用来处理系统的信号信息
		for {
			var sig os.Signal
			select {
			case s := <-signalCh:
				sig = s
			case <-ctx.Done():
				return
			}

			switch sig {
			case syscall.SIGPIPE:
				continue

			case syscall.SIGHUP:
				err := fmt.Errorf("cannot reload before agent started")
				c.logger.Error("Caught", "signal", sig, "error", err)

			default:
				c.logger.Info("Caught", "signal", sig)
				cancel()
				return
			}
		}
	}()

	err = agent.Start(ctx) // agent启动方法，在其中异步启动agent的service后退出
	signal.Stop(signalCh) // 服务已启动，无须再监听系统信号
	cancel() // 取消启动的context

	if err != nil {
		c.logger.Error("Error starting agent", "error", err)
		return 1 // 启动错误直接退出
	}

	// shutdown agent before endpoints
	defer agent.ShutdownEndpoints() //
	defer agent.ShutdownAgent()     // 关闭方法
	// 更新检查，仅在配置了更新检查的情况下启动，测试环境不启用
	if !config.DisableUpdateCheck && !config.DevMode {
		c.startupUpdateCheck(config)
	}
	// 将启动的agent加入到局域网中
	if err := c.startupJoin(agent, config); err != nil {
		c.logger.Error((err.Error()))
		return 1
	}

	// 将启动的agent加入到广域网中
	if err := c.startupJoinWan(agent, config); err != nil {
		c.logger.Error((err.Error()))
		return 1
	}

	// Let the agent know we've finished registration
	agent.StartSync() // 与远端进行信息同步

	cli.output("Consul agent running!")

	// wait for signal
	signalCh = make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	// 启动后监听系统信号，退出，异常等
	for {
		var sig os.Signal
		select {
		case s := <-signalCh:
			sig = s
		case <-service_os.Shutdown_Channel():
			sig = os.Interrupt
		case <-c.shutdownCh:
			sig = os.Interrupt
		case err := <-agent.RetryJoinCh(): //
			c.logger.Error("Retry join failed", "error", err)
			return 1
		case <-agent.Failed():
			// The deferred Shutdown method will log the appropriate error
			return 1
		case <-agent.ShutdownCh():
			// agent is already down!
			return 0
		}

		switch sig {
		case syscall.SIGPIPE:
			continue

		case syscall.SIGHUP:
			c.logger.Info("Caught", "signal", sig)

			err := agent.ReloadConfig()
			if err != nil {
				c.logger.Error("Reload config failed", "error", err)
			}
			config = agent.GetConfig()
		default:
			c.logger.Info("Caught", "signal", sig)
			// 平滑退出：退出前有序关闭各个子系统
			graceful := (sig == os.Interrupt && !(config.SkipLeaveOnInt)) || (sig == syscall.SIGTERM && (config.LeaveOnTerm))
			if !graceful {
				c.logger.Info("Graceful shutdown disabled. Exiting")
				return 1
			}

			c.logger.Info("Gracefully shutting down agent...")
			gracefulCh := make(chan struct{}) //
			go func() { // 调用agent.Leave()来进行平滑退出
				if err := agent.Leave(); err != nil {
					c.logger.Error("Error on leave", "error", err)
					return
				}
				close(gracefulCh)
			}()

			gracefulTimeout := 15 * time.Second
			select {
			case <-signalCh: // 连续收到两次退出信号会强制退出
				c.logger.Info("Caught second signal, Exiting", "signal", sig)
				return 1
			case <-time.After(gracefulTimeout): // 有序超时，强制退出
				c.logger.Info("Timeout on graceful leave. Exiting")
				return 1
			case <-gracefulCh: // 已完成平滑退出
				c.logger.Info("Graceful exit completed")
				return 0
			}
		}
	}
}

type GatedUi struct { // GateUi
	JSONoutput bool
	ui         cli.Ui
}

func (g *GatedUi) output(s string) {
	if !g.JSONoutput {
		g.ui.Output(s)
	}
}

func (g *GatedUi) info(s string) {
	if !g.JSONoutput {
		g.ui.Info(s)
	}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Runs a Consul agent" // agent命令解释说明
const help = `
Usage: consul agent [options]

  Starts the Consul agent and runs until an interrupt is received. The
  agent represents a single node in a cluster.
` // agent命令-h输出内容
