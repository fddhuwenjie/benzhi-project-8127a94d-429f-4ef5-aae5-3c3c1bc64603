package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
)

type config struct {
	Addr      string
	DataDir   string
	SelfCheck bool
}

func parseConfig(args []string) (config, error) {
	defaultAddr := "127.0.0.1:19081"
	if portText := os.Getenv("PORT"); portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return config{}, errors.New("PORT 必须是 1 到 65535 的端口号")
		}
		defaultAddr = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	set := flag.NewFlagSet("oralhistory", flag.ContinueOnError)
	var result config
	set.StringVar(&result.Addr, "addr", defaultAddr, "HTTP 监听地址")
	set.StringVar(&result.DataDir, "data-dir", "./oralhistory-data", "本地持久化目录")
	set.BoolVar(&result.SelfCheck, "self-check", false, "运行真实 HTTP 全流程自检后退出")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("存在无法识别的位置参数: %v", set.Args())
	}
	host, port, err := net.SplitHostPort(result.Addr)
	if err != nil {
		return config{}, fmt.Errorf("-addr 必须为 host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return config{}, errors.New("监听地址必须使用回环主机，禁止公开绑定")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 0 || portNumber > 65535 {
		return config{}, errors.New("监听端口无效")
	}
	return result, nil
}
