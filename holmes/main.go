package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"mosn.io/holmes"
	"mosn.io/holmes/reporters/pyroscope_reporter"
	"mosn.io/pkg/log"
)

func main() {
	InitHolmes("holmes-test")
	go func() {
		for {
			a := make([]byte, 1024*1024)
			fmt.Println("a len:", len(a))
			time.Sleep(2 * time.Second)
		}
	}()
	time.Sleep(2400 * time.Second)
}

func InitHolmes(appName string) {
	upstreamAddress := "http://127.0.0.1:4040"
	command := make([]string, 0, len(os.Args))
	args := make([]string, 0, len(os.Args))
	for _, v := range os.Args[1:] {
		if strings.HasPrefix(v, "-") {
			args = append(args, v)
		} else {
			command = append(command, v)
		}
	}
	cfg := pyroscope_reporter.RemoteConfig{
		// 上报地址
		UpstreamAddress: upstreamAddress,
		// 上报请求超时
		UpstreamRequestTimeout: 10 * time.Second,
	}
	hostname, _ := os.Hostname()
	// 要填自己pod的hostname 这样作为tag好排查
	tags := map[string]string{
		"hostname": hostname,
		"command":  strings.Join(command, "-"),
		"args":     strings.Join(args, "-"),
		"source":   "holmes",
		"service":  "holmes-test",
	}
	logger := holmes.NewStdLogger()
	logger.SetLogLevel(log.INFO) // 改为INFO级别，方便查看日志
	fmt.Println("正在初始化Pyroscope上报器...")
	pReporter, err := pyroscope_reporter.NewPyroscopeReporter(appName, tags, cfg, logger)
	if err != nil {
		fmt.Printf("NewPyroscopeReporter error %v\n", err)
		return
	}
	fmt.Println("Pyroscope上报器初始化成功，开始创建Holmes实例...")
	h, err := holmes.New(
		holmes.WithProfileReporter(pReporter),
		// holmes.WithDumpPath("/tmp"),
		holmes.WithDumpToLogger(true),
		holmes.WithMemoryLimit(10*1024*1024), // 降低到10MB以便更容易触发
		holmes.WithCPUCore(1),                // 单核也检测
		holmes.WithCPUDump(20, 30, 75, time.Minute),
		holmes.WithMemDump(10, 20, 50, time.Minute),             // 降低阈值以便更容易触发
		holmes.WithGoroutineDump(10, 20, 100, 500, time.Minute), // 降低阈值
		holmes.WithCollectInterval("5s"),                        // 缩短采集间隔
		holmes.WithLogger(logger),
	)
	if err != nil {
		fmt.Printf("创建Holmes实例失败: %v\n", err)
		return
	}
	fmt.Println("Holmes实例创建成功，开始启动监控...")
	h.EnableCPUDump().
		EnableGoroutineDump().
		EnableMemDump().
		Start()
	fmt.Println("Holmes监控已启动，数据将上报至:", upstreamAddress)
}
