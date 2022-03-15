`// +build <tag>` golang条件编译选项

`// +build windows` 表示仅在Windows上编译此文件
`// +build !windows` 表示在非Windows上才编译此文件

参考：https://www.cnblogs.com/FengZeng666/p/15689046.html


`// +build mytag` 可通过 `go build -tags=mytag .` 启用