package cmds

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/natefinch/lumberjack" // 用于滚动写入日志
	"github.com/urfave/cli"
)

type Log struct {
	VLevel          int
	VModule         string
	LogFile         string
	AlsoLogToStderr bool
}

var (
	LogConfig Log

	// 后期添加到 cli.Command.Flags即可
	VLevel = cli.IntFlag{
		Name:        "v",
		Usage:       "(logging) Number for the log level verbosity",
		Destination: &LogConfig.VLevel,
	}
	VModule = cli.StringFlag{
		Name:        "vmodule",
		Usage:       "(logging) Comma-separated list of pattern=N settings for file-filtered logging",
		Destination: &LogConfig.VModule,
	}
	LogFile = cli.StringFlag{
		Name:        "log,l",
		Usage:       "(logging) Log to file",
		Destination: &LogConfig.LogFile,
	}
	AlsoLogToStderr = cli.BoolFlag{
		Name:        "alsologtostderr",
		Usage:       "(logging) Log to standard error as well as file (if set)",
		Destination: &LogConfig.AlsoLogToStderr,
	}
)

// 当前进程用于记录日志，启动新的子进程执行功能（打印的数据都会由父进程接收），同时父进程wait()阻塞
func InitLogging() error {
	// $_K3S_LOG_REEXEC_=""即当前为父进程，$_K3S_LOG_REEXEC_=true时为子进程
	if LogConfig.LogFile != "" && os.Getenv("_K3S_LOG_REEXEC_") == "" {
		// 使用natefinch/lumberjack进行滚动日志记录
		// **当前为父进程**
		return runWithLogging()
	}

	// ***当前为子进程***
	// 检查服务器时间是否正确设置
	if err := checkUnixTimestamp(); err != nil {
		return err
	}

	// 使用golang默认日志记录工具
	setupLogging()
	return nil
}

// 一定程度判断服务器是否被配置过日期
func checkUnixTimestamp() error {
	timeNow := time.Now()
	// check if time before 01/01/1980
	// 此处目的不明，猜测为：如果系统从未设置时间，可能默认初始时间为01/01/1970。十年内（机器应用有效期）都能被系统判定正确（即未初始化时间）
	if timeNow.Before(time.Unix(315532800, 0)) {
		return fmt.Errorf("server time isn't set properly: %v", timeNow)
	}
	return nil
}

// 默认仅产生日志文件，若--{log|l}=true，则日志信息也会同时输出到标准输出流
func runWithLogging() error {
	var (
		l io.Writer
	)
	l = &lumberjack.Logger{
		Filename:   LogConfig.LogFile, // 指定文件名，例如： "/var/log/myapp/foo.log"
		MaxSize:    50,                // 最多写入50 Bytes的数据，默认100 Bytes
		MaxBackups: 3,                 // 最多保留3个旧的日志存档，默认所有（当然超出MaxAge还是会被删除）
		MaxAge:     28,                // 旧的日志最多保留28天，默认无期限
		Compress:   true,              // 滚动数据通过unzip压缩，默认不压缩
	}
	if LogConfig.AlsoLogToStderr {
		l = io.MultiWriter(l, os.Stderr)
	}

	args := append([]string{"k3s"}, os.Args[1:]...)
	cmd := reexec.Command(args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "_K3S_LOG_REEXEC_=true") // 添加$_K3S_LOG_REEXEC_=true环境变量，标志当前进程为父进程，用于日志记录
	cmd.Stderr = l
	cmd.Stdout = l
	cmd.Stdin = os.Stdin
	return cmd.Run() // 相当于新开进程执行 $exec k3s os.Args[1:]... , 此函数会阻塞（父进程阻塞）直到程序执行完毕，日志会全部输出到 变量l 指定的io.Writer
}

// 在--debug=true的情况下，会同时生成日志文件和终端打印信息，--debug=false时，仅输出日志到标准输出流
// 标准输出流数据实际上被父进程接收，会被记录到日志文件
func setupLogging() {
	// 有疑问可查阅文件底部注释内容
	flag.Set("v", strconv.Itoa(LogConfig.VLevel))
	flag.Set("vmodule", LogConfig.VModule)
	flag.Set("alsologtostderr", strconv.FormatBool(debug)) // 设置为true时，即希望生成日志文件，同时在标准输出流输出日志信息
	flag.Set("logtostderr", strconv.FormatBool(!debug))    // 设置为true时，日志仅记录到标准输出流，而不会写入日志文件（最高优先级判定）
}

/***************************************关于 flag包内置的环境变量（日志相关）***************************

--add_dir_header
		If true, adds the file directory to the header of the log messages
--alsologtostderr
		log to standard error as well as files
--log_backtrace_at traceLocation
		when logging hits line file:N, emit a stack trace (default :0)
--log_dir string
		If non-empty, write log files in this directory
--log_file string
		If non-empty, use this log file
--log_file_max_size uint
		Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited.
--logtostderr
		log to standard error instead of files
--one_output
		If true, only write logs to their native severity level (vs also writing to each lower severity level)
--skip_headers
		If true, avoid header prefixes in the log messages
--skip_log_headers
		If true, avoid headers when opening log files
--stderrthreshold severity
		logs at or above this threshold go to stderr (default 2)
-v, --v Level
		number for the log level verbosity
--vmodule moduleSpec
		comma-separated list of pattern=N settings for file-filtered logging

******************************************************************************************************/
