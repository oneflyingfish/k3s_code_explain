阅读本篇之前，强烈建议先浏览[k3s描述](cmd/k3s/readme.md)

| 类型 | 形式 | 实际执行 | 附加 |
| :-: | :- | :- | :- | 
| 重命名 | kubectl, crictl, ctr，containerd | 第三方已有Cli程序 | 可能略微添加自动的环境变量处理等 |
| 子命令 | $exec {kubectl, crictl, ctr } | 第三方已有Cli程序 | 与重命名调用完全同样的代码 |
| 子命令 | $exec {server, agent} | k3s核心组件 | - |


