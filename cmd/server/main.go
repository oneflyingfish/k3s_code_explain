package main

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec" // 借助docker实现的reexec package，类似C语言的fork功能，实现golang多进程编程（注意与线程、go routine区分），参考：https://jiajunhuang.com/articles/2018_03_08-golang_fork.md.html
	crictl2 "github.com/kubernetes-sigs/cri-tools/cmd/crictl"
	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/crictl"
	"github.com/rancher/k3s/pkg/cli/ctr"
	"github.com/rancher/k3s/pkg/cli/kubectl"
	"github.com/rancher/k3s/pkg/cli/server"
	"github.com/rancher/k3s/pkg/containerd"
	ctr2 "github.com/rancher/k3s/pkg/ctr"
	kubectl2 "github.com/rancher/k3s/pkg/kubectl"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func init() {
	reexec.Register("containerd", containerd.Main)
	reexec.Register("kubectl", kubectl2.Main)
	reexec.Register("crictl", crictl2.Main)
	reexec.Register("ctr", ctr2.Main)
}

// 例如执行 /.../$EXEC args
func main() {
	cmd := os.Args[0]
	os.Args[0] = filepath.Base(os.Args[0]) // 去除path路径前缀，即为$EXEC

	// 虽然用到了reexec，但是此处并未开启多进程
	if reexec.Init() { // .Init()内部判断os.Args[0]是否已经通过reexec.Register注册，注册则执行对应函数，返回true，否则返回false
		// 根据 $EXEC 匹配 {containerd|kubectl|crictl|ctr}，执行上述reexec.Register注册的函数，例如  containerd => containerd.Main()
		// 相当于执行 shell `containerd args``
		return
	}

	// 非注册程序
	os.Args[0] = cmd

	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(server.Run),
		cmds.NewAgentCommand(agent.Run),
		cmds.NewKubectlCommand(kubectl.Run),
		cmds.NewCRICTL(crictl.Run),
		cmds.NewCtrCommand(ctr.Run),
	}

	err := app.Run(os.Args) // 子命令方式执行 /.../$EXEC args， 例如此处可能匹配到 /.../exec_name kubectl --help
	if err != nil {
		logrus.Fatal(err)
	}
}
