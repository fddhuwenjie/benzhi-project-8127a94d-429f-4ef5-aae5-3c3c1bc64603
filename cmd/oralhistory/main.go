package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	cfg, err := parseConfig(os.Args[1:])
	if err == nil {
		if cfg.SelfCheck {
			err = runSelfCheck(cfg)
		} else {
			err = runServer(cfg)
		}
	}
	if err != nil {
		log.Printf("启动失败: %v", err)
		os.Exit(1)
	}
}
