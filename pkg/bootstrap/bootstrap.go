package bootstrap

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
)

func Handler(bootstrap *config.ControlRuntimeBootstrap) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		Write(rw, bootstrap)
	})
}

// 存在本地文件路径为path，数据为data
// bootstrap ==> filekey: path  [以此filekey为遍历对象]
// 将数据写入到w, 使得 w ==> filekey: data
func Write(w io.Writer, bootstrap *config.ControlRuntimeBootstrap) error {
	paths, err := objToMap(bootstrap)
	if err != nil {
		return nil
	}

	dataMap := map[string][]byte{}
	for pathKey, path := range paths {
		if path == "" {
			continue
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to read %s", path)
		}

		dataMap[pathKey] = data
	}

	return json.NewEncoder(w).Encode(dataMap)
}

// r ==> filekey: data  [以此filekey为遍历对象]
// bootstrap ==> filekey: path
// 功能：将 "data" 写入到本地的 "path"路径
func Read(r io.Reader, bootstrap *config.ControlRuntimeBootstrap) error {
	paths, err := objToMap(bootstrap) // filekey: path
	if err != nil {
		return err
	}

	files := map[string][]byte{} // filekey: data
	if err := json.NewDecoder(r).Decode(&files); err != nil {
		return err
	}

	for pathKey, data := range files {
		path, ok := paths[pathKey]
		if !ok {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return errors.Wrapf(err, "failed to mkdir %s", filepath.Dir(path))
		}

		if err := ioutil.WriteFile(path, data, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", path)
		}
	}

	return nil
}

func objToMap(obj interface{}) (map[string]string, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	return data, json.Unmarshal(bytes, &data)
}
