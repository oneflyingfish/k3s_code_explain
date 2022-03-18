package rootless

import (
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rootless-containers/rootlesskit/pkg/child"
	"github.com/rootless-containers/rootlesskit/pkg/copyup/tmpfssymlink"
	"github.com/rootless-containers/rootlesskit/pkg/network/slirp4netns"
	"github.com/rootless-containers/rootlesskit/pkg/parent"
	portbuiltin "github.com/rootless-containers/rootlesskit/pkg/port/builtin"
	"github.com/sirupsen/logrus"
)

var (
	pipeFD   = "_K3S_ROOTLESS_FD"
	childEnv = "_K3S_ROOTLESS_SOCK"
	Sock     = ""
)

// 默认: stateDir= ${HOME}/.rancher/k3s
func Rootless(stateDir string) error {
	defer func() {
		os.Unsetenv(pipeFD)
		os.Unsetenv(childEnv)
	}()

	hasFD := os.Getenv(pipeFD) != ""
	hasChildEnv := os.Getenv(childEnv) != ""

	// 注意，初次执行此函数，hasFD=false
	if hasFD {
		// $_K3S_ROOTLESS_FD存在
		// 此时运行的是子进程
		logrus.Debug("Running rootless child")
		childOpt, err := createChildOpt()
		if err != nil {
			logrus.Fatal(err)
		}

		// 内部会接收parent发送的套接字信息，判断message，如果parent状态还未就绪会再次重启（直接通过syscall.exec替换当前进程）
		// 状态就绪后，阻塞当前进程（实际上已经是子进程），启动孙子进程（/proc/self/exe os.args[1:]...），执行程序
		if err := child.Child(*childOpt); err != nil {
			logrus.Fatal("child died", err)
		}
	}

	// 注意，初次执行次函数，hasChildEnv=false
	if hasChildEnv {
		// 此时子进程初始化成功
		// $_K3S_ROOTLESS_SOCK存在
		Sock = os.Getenv(childEnv)
		logrus.Debug("Running rootless process")
		return setupMounts(stateDir)
	}

	logrus.Debug("Running rootless parent")
	parentOpt, err := createParentOpt(filepath.Join(stateDir, "rootless")) // ${HOME}/.rancher/k3s/rootless
	if err != nil {
		logrus.Fatal(err)
	}

	os.Setenv(childEnv, filepath.Join(parentOpt.StateDir, parent.StateFileAPISock)) // $_K3S_ROOTLESS_SOCK={REST API Socket}
	// 此处会阻塞，内部开始监听Sock，将以子进程运行 /proc/self/exe os.Args[1:]
	// /proc/self/exe 为特殊路径，指向当前程序的路径
	if err := parent.Parent(*parentOpt); err != nil {
		logrus.Fatal(err)
	}
	os.Exit(0)

	return nil
}

func parseCIDR(s string) (*net.IPNet, error) {
	if s == "" {
		return nil, nil
	}
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	if !ip.Equal(ipnet.IP) {
		return nil, errors.Errorf("cidr must be like 10.0.2.0/24, not like 10.0.2.100/24")
	}
	return ipnet, nil
}

func createParentOpt(stateDir string) (*parent.Opt, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %s", stateDir)
	}

	stateDir, err := ioutil.TempDir("", "rootless") // 在os.TempDir目录下创建以rootless为前缀的文件夹 (rootless$random)，返回值为新文件夹的path
	if err != nil {
		return nil, err
	}

	opt := &parent.Opt{
		StateDir: stateDir,
	}

	mtu := 0
	ipnet, err := parseCIDR("10.41.0.0/16")
	if err != nil {
		return nil, err
	}
	disableHostLoopback := true // 即禁止连接主机命名空间的127.0.0.1:*
	binary := "slirp4netns"

	// 确保slirp4netns已经安装并配置环境变量
	// 在环境变量中查找slirp4netns二进制文件，不存在直接返回错误，第一个返回值为 slirp4netns的path，例如: /bin/slirp4netns
	if _, err := exec.LookPath(binary); err != nil {
		return nil, err
	}
	opt.NetworkDriver = slirp4netns.NewParentDriver(binary, mtu, ipnet, disableHostLoopback, "")
	opt.PortDriver, err = portbuiltin.NewParentDriver(&logrusDebugWriter{}, stateDir)
	if err != nil {
		return nil, err
	}

	opt.PipeFDEnvKey = pipeFD

	return opt, nil
}

type logrusDebugWriter struct {
}

func (w *logrusDebugWriter) Write(p []byte) (int, error) {
	s := strings.TrimSuffix(string(p), "\n")
	logrus.Debug(s)
	return len(p), nil
}

func createChildOpt() (*child.Opt, error) {
	opt := &child.Opt{}
	opt.TargetCmd = os.Args
	opt.PipeFDEnvKey = pipeFD
	opt.NetworkDriver = slirp4netns.NewChildDriver()
	opt.PortDriver = portbuiltin.NewChildDriver(&logrusDebugWriter{})
	opt.CopyUpDirs = []string{"/etc", "/run", "/var/lib"}
	opt.CopyUpDriver = tmpfssymlink.NewChildDriver()
	return opt, nil
}
