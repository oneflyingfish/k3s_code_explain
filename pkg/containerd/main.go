//go:build ctrd
// +build ctrd

package containerd

import (
	"fmt"
	"os"

	"github.com/containerd/containerd/cmd/containerd/command"
	"github.com/containerd/containerd/pkg/seed"
)

// 直接运行官方containerd
func Main() {
	//klog.InitFlags(nil)
	seed.WithTimeAndRand()
	app := command.App()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "containerd: %s\n", err)
		os.Exit(1)
	}
}
