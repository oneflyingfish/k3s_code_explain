package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/data" // 基于go-bindata生成的文件，静态嵌套
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/untar"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	if runCLIs() {
		return
	}

	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(wrap("k3s-server", os.Args)),
		cmds.NewAgentCommand(wrap("k3s-agent", os.Args)),
		cmds.NewKubectlCommand(externalCLIAction("kubectl")),
		cmds.NewCRICTL(externalCLIAction("crictl")),
		cmds.NewCtrCommand(externalCLIAction("ctr")),
		cmds.NewCheckConfigCommand(externalCLIAction("check-config")),
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}

// k3s {kubectl|ctr|crictl} 直接在当前目录执行，成功返回true
func runCLIs() bool {
	for _, cmd := range []string{"kubectl", "ctr", "crictl"} {
		if filepath.Base(os.Args[0]) == cmd {
			if err := externalCLI(cmd, "", os.Args[1:]); err != nil {
				logrus.Fatal(err)
			}
			return true
		}
	}
	return false
}

func externalCLIAction(cmd string) func(cli *cli.Context) error {
	return func(cli *cli.Context) error {
		return externalCLI(cmd, cli.String("data-dir"), cli.Args())
	}
}

func externalCLI(cli, dataDir string, args []string) error {
	dataDir, err := datadir.Resolve(dataDir)
	if err != nil {
		return err
	}
	return stageAndRun(dataDir, cli, append([]string{cli}, args...))
}

// wrap和externalCLIAction的区别在于，args是来自于k3s命令（wrap），还是其子命令（externalCLIAction）
func wrap(cmd string, args []string) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		return stageAndRunCLI(ctx, cmd, args) // 定义函数闭包
	}
}

func stageAndRunCLI(cli *cli.Context, cmd string, args []string) error {
	dataDir, err := datadir.Resolve(cli.String("data-dir")) // 读取flag: data-dir的值设置为工作目录。default: 普通用户为 ~/.rancher/k3s, root用户为 /var/lib/rancher/k3s
	if err != nil {
		return err
	}

	// 此时datadir指向一个给定的文件夹，应为绝对路径
	return stageAndRun(dataDir, cmd, args)
}

// 将静态资源解压到 dataDir/data/$HASH（内含cmd命令和若干系统命令，配置临时环境变量），执行 shell `cmd args`
func stageAndRun(dataDir string, cmd string, args []string) error {
	dir, err := extract(dataDir) // 将静态嵌套资源 $HASH.tar.gz 解压到 dataDir/data/$HASH
	if err != nil {
		return errors.Wrap(err, "extracting data")
	}

	// [仅对当前程序生效] 环境变量添加 $PATH = dataDir/data/$HASH/bin:dataDir/data/$HASH/bin/aux
	if err := os.Setenv("PATH", filepath.Join(dir, "bin")+":"+os.Getenv("PATH")+":"+filepath.Join(dir, "bin/aux")); err != nil {
		return err
	}

	// [仅对当前程序生效] 添加 $K3S_DATA_DIR = dataDir/data/$HASH
	if err := os.Setenv("K3S_DATA_DIR", dir); err != nil {
		return err
	}

	cmd, err = exec.LookPath(cmd) // 环境变量中查找命令
	if err != nil {
		return err
	}

	logrus.Debugf("Running %s %v", cmd, args)
	return syscall.Exec(cmd, args, os.Environ()) // 执行命令
}

func getAssetAndDir(dataDir string) (string, string) {
	asset := data.AssetNames()[0] // 只有一个文件，即为 $HASH.tar.gz
	dir := filepath.Join(dataDir, "data", strings.SplitN(filepath.Base(asset), ".", 2)[0])

	// filename.tar.gz, $dataDir/data/filename
	return asset, dir
}

// 将静态嵌套资源 $HASH.tar.gz 解压到 dataDir/data/$HASH
func extract(dataDir string) (string, error) {
	// first look for global asset folder so we don't create a HOME version if not needed
	_, dir := getAssetAndDir(datadir.DefaultDataDir) // default: dir = /var/lib/rancher/k3s/data/filename, 实际测试filename为HASH值
	if _, err := os.Stat(dir); err == nil {
		// root用户文件夹已经存在，不再继续执行后续
		logrus.Debugf("Asset dir %s", dir)
		return dir, nil
	}

	asset, dir := getAssetAndDir(dataDir) // default: asset=filename.tar.gz , dir=/var/lib/rancher/k3s/data/filename
	if _, err := os.Stat(dir); err == nil {
		logrus.Debugf("Asset dir %s", dir)
		return dir, nil
	}

	logrus.Infof("Preparing data dir %s", dir)

	content, err := data.Asset(asset) // 从go-bindata读取嵌入资源 filename.tar.gz
	if err != nil {
		return "", err
	}

	buf := bytes.NewBuffer(content)

	tempDest := dir + "-tmp"
	defer os.RemoveAll(tempDest)

	os.RemoveAll(tempDest) // default: tempDest = /var/lib/rancher/k3s/data/filename-temp

	// 解压到：/var/lib/rancher/k3s/data/filename-temp
	if err := untar.Untar(buf, tempDest); err != nil {
		return "", err
	}

	// filename-temp重命名为filename
	return dir, os.Rename(tempDest, dir) // /var/lib/rancher/k3s/data/filename, err
}
