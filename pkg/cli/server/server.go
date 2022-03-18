package server

import (
	"context"
	"fmt"
	net2 "net"
	"os"
	"path/filepath"
	"strings"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/netutil"
	"github.com/rancher/k3s/pkg/rootless"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/k3s/pkg/token"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/pkg/master"

	_ "github.com/go-sql-driver/mysql" // ensure we have mysql
	_ "github.com/lib/pq"              // ensure we have postgres
	_ "github.com/mattn/go-sqlite3"    // ensure we have sqlite
)

func Run(app *cli.Context) error {
	// 以"k3s"为进程名，在子进程执行功能的进程中功能，cmds.InitLogging()将会阻塞父进程，直到调用子进程退出.
	// LogConfig.LogFile != ""时将会启用
	if err := cmds.InitLogging(); err != nil {
		return err
	}

	// 子进程cmds.InitLogging()直接返回nil，顺利执行run函数，开始处理事务
	// 父进程在子进程结束后，疑似会再次执行run????? 此处疑似存在逻辑bug
	// 备注：子进程正常执行，且os.exit(0)时，cmds.InitLogging()返回值将为nil，由此触发run函数二次执行
	return run(app, &cmds.ServerConfig)
}

func run(app *cli.Context, cfg *cmds.Server) error {
	var (
		err error
	)

	// 对于非root以用户，--rootless=false --disable-agent=false 会引发error
	if !cfg.DisableAgent && os.Getuid() != 0 && !cfg.Rootless {
		return fmt.Errorf("must run as root unless --disable-agent is specified")
	}

	if cfg.Rootless {
		dataDir, err := datadir.LocalHome(cfg.DataDir, true)
		if err != nil {
			return err
		}
		cfg.DataDir = dataDir

		// 内部会阻塞当前进程，通过子孙进程的方式，实现rootless
		if err := rootless.Rootless(dataDir); err != nil {
			return err
		}

		// 启动成功后，孙进程此处err==nil
		// 此处 os.Getuid()==0  # fake root, 是通过rootless实现的
	}

	if cfg.Token == "" && cfg.ClusterSecret != "" {
		cfg.Token = cfg.ClusterSecret
	}

	// 准备启动server的参数
	// type Config struct {
	// 	DisableAgent     bool
	// 	DisableServiceLB bool
	// 	ControlConfig    config.Control
	// 	Rootless         bool
	// }
	// cfg为命令行输入flags填充
	serverConfig := server.Config{}
	serverConfig.DisableAgent = cfg.DisableAgent
	serverConfig.ControlConfig.Token = cfg.Token
	serverConfig.ControlConfig.AgentToken = cfg.AgentToken
	serverConfig.ControlConfig.JoinURL = cfg.ServerURL
	if cfg.AgentTokenFile != "" {
		serverConfig.ControlConfig.AgentToken, err = token.ReadFile(cfg.AgentTokenFile)
		if err != nil {
			return err
		}
	}
	if cfg.TokenFile != "" {
		serverConfig.ControlConfig.Token, err = token.ReadFile(cfg.TokenFile)
		if err != nil {
			return err
		}
	}
	serverConfig.ControlConfig.DataDir = cfg.DataDir
	serverConfig.ControlConfig.KubeConfigOutput = cfg.KubeConfigOutput
	serverConfig.ControlConfig.KubeConfigMode = cfg.KubeConfigMode
	serverConfig.ControlConfig.NoScheduler = cfg.DisableScheduler
	serverConfig.Rootless = cfg.Rootless
	serverConfig.ControlConfig.SANs = knownIPs(cfg.TLSSan)
	serverConfig.ControlConfig.BindAddress = cfg.BindAddress
	serverConfig.ControlConfig.HTTPSPort = cfg.HTTPSPort
	serverConfig.ControlConfig.ExtraAPIArgs = cfg.ExtraAPIArgs
	serverConfig.ControlConfig.ExtraControllerArgs = cfg.ExtraControllerArgs
	serverConfig.ControlConfig.ExtraSchedulerAPIArgs = cfg.ExtraSchedulerArgs
	serverConfig.ControlConfig.ClusterDomain = cfg.ClusterDomain
	serverConfig.ControlConfig.Datastore.Endpoint = cfg.DatastoreEndpoint
	serverConfig.ControlConfig.Datastore.CAFile = cfg.DatastoreCAFile
	serverConfig.ControlConfig.Datastore.CertFile = cfg.DatastoreCertFile
	serverConfig.ControlConfig.Datastore.KeyFile = cfg.DatastoreKeyFile
	serverConfig.ControlConfig.AdvertiseIP = cfg.AdvertiseIP
	serverConfig.ControlConfig.AdvertisePort = cfg.AdvertisePort
	serverConfig.ControlConfig.FlannelBackend = cfg.FlannelBackend
	serverConfig.ControlConfig.ExtraCloudControllerArgs = cfg.ExtraCloudControllerArgs
	serverConfig.ControlConfig.DisableCCM = cfg.DisableCCM
	serverConfig.ControlConfig.DisableNPC = cfg.DisableNPC
	serverConfig.ControlConfig.ClusterInit = cfg.ClusterInit
	serverConfig.ControlConfig.ClusterReset = cfg.ClusterReset

	if cmds.AgentConfig.FlannelIface != "" && cmds.AgentConfig.NodeIP == "" {
		cmds.AgentConfig.NodeIP = netutil.GetIPFromInterface(cmds.AgentConfig.FlannelIface)
	}

	if serverConfig.ControlConfig.AdvertiseIP == "" && cmds.AgentConfig.NodeExternalIP != "" {
		serverConfig.ControlConfig.AdvertiseIP = cmds.AgentConfig.NodeExternalIP
	}
	if serverConfig.ControlConfig.AdvertiseIP == "" && cmds.AgentConfig.NodeIP != "" {
		serverConfig.ControlConfig.AdvertiseIP = cmds.AgentConfig.NodeIP
	}
	if serverConfig.ControlConfig.AdvertiseIP != "" {
		serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, serverConfig.ControlConfig.AdvertiseIP)
	}

	_, serverConfig.ControlConfig.ClusterIPRange, err = net2.ParseCIDR(cfg.ClusterCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ClusterCIDR, err)
	}
	_, serverConfig.ControlConfig.ServiceIPRange, err = net2.ParseCIDR(cfg.ServiceCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ServiceCIDR, err)
	}

	_, apiServerServiceIP, err := master.DefaultServiceIPRange(*serverConfig.ControlConfig.ServiceIPRange)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, apiServerServiceIP.String())

	// If cluster-dns CLI arg is not set, we set ClusterDNS address to be ServiceCIDR network + 10,
	// i.e. when you set service-cidr to 192.168.0.0/16 and don't provide cluster-dns, it will be set to 192.168.0.10
	if cfg.ClusterDNS == "" {
		serverConfig.ControlConfig.ClusterDNS = make(net2.IP, 4)
		copy(serverConfig.ControlConfig.ClusterDNS, serverConfig.ControlConfig.ServiceIPRange.IP.To4())
		serverConfig.ControlConfig.ClusterDNS[3] = 10
	} else {
		serverConfig.ControlConfig.ClusterDNS = net2.ParseIP(cfg.ClusterDNS)
	}

	if cfg.DefaultLocalStoragePath == "" {
		dataDir, err := datadir.LocalHome(cfg.DataDir, false)
		if err != nil {
			return err
		}
		serverConfig.ControlConfig.DefaultLocalStoragePath = filepath.Join(dataDir, "/storage")
	} else {
		serverConfig.ControlConfig.DefaultLocalStoragePath = cfg.DefaultLocalStoragePath
	}

	noDeploys := make([]string, 0)

	// 假设: ./exec --no-deploy=coredns,servicelb  --no-deploy=local-storage,metrics-server
	// 那么: app.StringSlice("no-deploy")的返回值为 ["coredns,servicelb", "local-storage,metrics-server"]
	// 最终: noDeploys=["coredns", "servicelb","local-storage", "metrics-server"]
	for _, noDeploy := range app.StringSlice("no-deploy") {
		// 原版代码，被替换
		// for _, splitNoDeploy := range strings.Split(noDeploy, ",") {
		// 	noDeploys = append(noDeploys, splitNoDeploy)
		// }

		noDeploys = append(noDeploys, strings.Split(noDeploy, ",")...) // 此处优化了原版的代码，不影响语义，可选项<coredns, servicelb, traefik, local-storage, metrics-server>
	}

	for _, noDeploy := range noDeploys {
		if noDeploy == "servicelb" {
			serverConfig.DisableServiceLB = true
			continue
		}
		serverConfig.ControlConfig.Skips = append(serverConfig.ControlConfig.Skips, noDeploy)
	}

	logrus.Info("Starting k3s ", app.App.Version)
	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	ctx := signals.SetupSignalHandler(context.Background())
	if err := server.StartServer(ctx, &serverConfig); err != nil {
		return err
	}

	logrus.Info("k3s is up and running")
	if notifySocket != "" {
		os.Setenv("NOTIFY_SOCKET", notifySocket)

		// 注意：$NOTIFY_SOCKET是systemd.SdNotify内置环境变量，与k3s设计无关
		// true表示无条件unset $NOTIFY_SOCKET
		// 向初始化当前进程的进程(k3s中描述为父进程)发送信息: "READY=1\n"，此字符串为操作系统约定字段，有一定格式
		systemd.SdNotify(true, "READY=1\n")
	}

	if cfg.DisableAgent {
		<-ctx.Done()
		return nil
	}

	// 默认在server节点，也会启动一个agent。可以通过--disable-agent=true屏蔽
	ip := serverConfig.ControlConfig.BindAddress
	if ip == "" {
		ip = "127.0.0.1"
	}

	url := fmt.Sprintf("https://%s:%d", ip, serverConfig.ControlConfig.HTTPSPort)
	token, err := server.FormatToken(serverConfig.ControlConfig.Runtime.AgentToken, serverConfig.ControlConfig.Runtime.ServerCA)
	if err != nil {
		return err
	}

	agentConfig := cmds.AgentConfig
	agentConfig.Debug = app.GlobalBool("bool")
	agentConfig.DataDir = filepath.Dir(serverConfig.ControlConfig.DataDir)
	agentConfig.ServerURL = url
	agentConfig.Token = token
	agentConfig.DisableLoadBalancer = true
	agentConfig.Rootless = cfg.Rootless
	if agentConfig.Rootless {
		// let agent specify Rootless kubelet flags, but not unshare twice
		agentConfig.RootlessAlreadyUnshared = true
	}

	return agent.Run(ctx, agentConfig)
}

func knownIPs(ips []string) []string {
	ips = append(ips, "127.0.0.1")
	ip, err := net.ChooseHostInterface()
	if err == nil {
		ips = append(ips, ip.String())
	}
	return ips
}
