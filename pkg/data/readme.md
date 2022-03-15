data.go全部内容由oneflyingfish手动构建，不保证与官方包一致，压缩包根据代码逻辑从官方二进制逆向获得...

过程:

```bash
# k3s下载地址：https://github.com/k3s-io/k3s/releases/download/v1.0.0/k3s
# 解压路径为 /var/lib/rancher/k3s/data/4012316506613ee8c3cffc1e5b5eca706270685d33585804b257e93ea98d1917（执行sudo ./k3s --debug kubectl --help)即可生成,会自动打印解压路径， k3s项目地址为 ~/k3s/，请自行修正为自己电脑上的

# 逆向获取 4012316506613ee8c3cffc1e5b5eca706270685d33585804b257e93ea98d1917.tar.gz 并移动到 ~/k3s/pkg/data/data
cd /var/lib/rancher/k3s/data/4012316506613ee8c3cffc1e5b5eca706270685d33585804b257e93ea98d1917
sudo tar zcf 4012316506613ee8c3cffc1e5b5eca706270685d33585804b257e93ea98d1917.tar.gz  *
mkdir -p ~/k3s/pkg/data/data
sudo mv 4012316506613ee8c3cffc1e5b5eca706270685d33585804b257e93ea98d1917.tar.gz ~/k3s/pkg/data/data

# 自行安装 go-bindata
cd ~/k3s/pkg/data
go-bindata -pkg=data -o=./data.go -prefix=./data/ ./data/...    # 即可看到~/k3s/pkg/data/data.go

# 此时可以顺利编译k3s
cd ~/k3s/cmd/k3s && go build .
```
