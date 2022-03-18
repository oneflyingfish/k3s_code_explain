## RootlessKit介绍：

> 推荐阅读：https://zhuanlan.zhihu.com/p/64661985
> 
> 有关`veth pair(Virtual Ethernet pair)`的内容: https://zhuanlan.zhihu.com/p/185686233
>
> rootless下的网络问题解决: https://www.sohu.com/a/405090353_354899

* 类型: 基于用户命名空间的Linux fakeroot
* 目标: 以非root用户运行Docker和Kubernetes ，从而保护主机上的真正根免受潜在的容器突破攻击，项目地址：https://github.com/rootless-containers/rootlesskit
* 类似项目: fakeroot、fakeroot-ng、proot、become-root等等

## 网络层转发方案:
* host
* **slirp4netns** (k3s选用的方案)
* vpnkit
* lxc-user-nic (实验性)
> MTU=0指默认值，slirp4netns默认是65520

## 端口转发方案:
* none
* **buildin** (k3s选用的方案)
* slirp4netns

## k3s的rootless包

如果你是第一次接触rootless技术以及对容器技术的本质完全不了解，这个包的代码理解起来可能有一点晤涩难懂。一句话解释其实就是利用rootless技术，让程序在非root的情况下以虚假的root的形式运行起来(此时查看os.Getuid()==0)。