package passwd

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rancher/k3s/pkg/token"
)

type entry struct {
	pass string
	role string
}

type Passwd struct {
	changed bool
	names   map[string]entry
}

func Read(file string) (*Passwd, error) {
	result := &Passwd{
		names: map[string]entry{},
	}

	f, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 2 {
			return nil, fmt.Errorf("password file '%s' must have at least 2 columns (password, name), found %d", file, len(record))
		}
		e := entry{
			pass: record[0],
		}
		if len(record) > 3 {
			e.role = record[3]
		}
		result.names[record[1]] = e
	}

	return result, nil
}

func (p *Passwd) Check(name, pass string) (matches bool, exists bool) {
	e, ok := p.names[name]
	if !ok {
		return false, false
	}
	return e.pass == pass, true
}

// 用role和passwd更新/创建 passwd[name]
// passwd为空时，表示保持原值不变/随机生成(不存在原值时)
func (p *Passwd) EnsureUser(name, role, passwd string) error {
	tokenPrefix := "::" + name + ":"
	idx := strings.Index(passwd, tokenPrefix)
	// {...}括号为标记方便引入，非实际存在的字符
	// 当 pass为 k10{A}::{name}:{B}，则更新pass为 {B}
	if idx > 0 && strings.HasPrefix(passwd, "K10") {
		passwd = passwd[idx+len(tokenPrefix):]
	}

	// name的pass已经存在
	if e, ok := p.names[name]; ok {
		if passwd != "" && e.pass != passwd {
			p.changed = true
			e.pass = passwd
		}

		if e.role != role {
			p.changed = true
			e.role = role
		}

		p.names[name] = e
		return nil
	}

	// name不存在
	// 若pass为""则直接随机生成
	if passwd == "" {
		token, err := token.Random(16)
		if err != nil {
			return err
		}
		passwd = token
	}

	p.changed = true
	p.names[name] = entry{
		pass: passwd,
		role: role,
	}

	return nil
}

func (p *Passwd) Pass(name string) (string, bool) {
	e, ok := p.names[name]
	if !ok {
		return "", false
	}
	return e.pass, true
}

func (p *Passwd) Write(passwdFile string) error {
	if !p.changed {
		return nil
	}

	var records [][]string
	for name, e := range p.names {
		records = append(records, []string{
			e.pass,
			name,
			name,
			e.role,
		})
	}

	return writePasswords(passwdFile, records)
}

func writePasswords(passwdFile string, records [][]string) error {
	out, err := os.Create(passwdFile + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	if err := out.Chmod(0600); err != nil {
		return err
	}

	if err := csv.NewWriter(out).WriteAll(records); err != nil {
		return err
	}

	return os.Rename(passwdFile+".tmp", passwdFile)
}
