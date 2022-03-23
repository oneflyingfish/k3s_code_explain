package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/sirupsen/logrus"
)

// 初始化c.storageClient，用于读写数据库
// 并判断是否有c.config.Token对应的条目。
// 如果已经存在，读取bootstrap信息，并在本地生成
// 如果不存在，则标记c.saveBootstrap=true
func (c *Cluster) Join(ctx context.Context) error {
	runJoin, err := c.shouldJoin() // 默认为true, nil
	if err != nil {
		return err
	}
	c.runJoin = runJoin

	if runJoin {
		if err := c.join(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *Cluster) shouldJoin() (bool, error) {
	dqlite := c.dqliteEnabled() // 默认为false
	if dqlite {
		c.runtime.HTTPBootstrap = true
		if c.config.JoinURL == "" {
			return false, nil
		}
	}

	stamp := c.joinStamp() // Datadir/db/join-hash(c.config.Token)
	if _, err := os.Stat(stamp); err == nil {
		logrus.Info("Cluster bootstrap already complete")
		return false, nil
	}

	if dqlite && c.config.Token == "" {
		return false, fmt.Errorf("K3S_TOKEN is required to join a cluster")
	}

	return true, nil
}

func (c *Cluster) joined() error {
	if err := os.MkdirAll(filepath.Dir(c.joinStamp()), 0700); err != nil {
		return err
	}

	if _, err := os.Stat(c.joinStamp()); err == nil {
		return nil
	}

	f, err := os.Create(c.joinStamp())
	if err != nil {
		return err
	}

	return f.Close()
}

func (c *Cluster) httpJoin() error {
	token, err := clientaccess.NormalizeAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "server")
	if err != nil {
		return err
	}

	info, err := clientaccess.ParseAndValidateToken(c.config.JoinURL, token)
	if err != nil {
		return err
	}
	c.clientAccessInfo = info

	content, err := clientaccess.Get("/v1-k3s/server-bootstrap", info)
	if err != nil {
		return err
	}

	return bootstrap.Read(bytes.NewBuffer(content), &c.runtime.ControlRuntimeBootstrap)
}

func (c *Cluster) join(ctx context.Context) error {
	c.joining = true

	if c.runtime.HTTPBootstrap { // 使用dqlite时此值为true
		return c.httpJoin()
	}

	// 初始化c.storageClient，用于读写数据库
	// 并判断是否有c.config.Token对应的条目。
	// 如果已经存在，读取bootstrap信息，并在本地生成
	// 如果不存在，则标记c.saveBootstrap=true
	if err := c.storageJoin(ctx); err != nil {
		return err
	}

	return nil
}

// return "Datadir/db/join-hash(c.config.Token)"
func (c *Cluster) joinStamp() string {
	return filepath.Join(c.config.DataDir, "db/joined-"+keyHash(c.config.Token))
}
