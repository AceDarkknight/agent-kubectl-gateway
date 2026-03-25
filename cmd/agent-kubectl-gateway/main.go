package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/audit"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/executor"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/filter"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/server"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "configs/config.yaml", "Path to the configuration file")
	flag.Parse()

	// 检查配置文件是否存在
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Fatalf("Config file not found: %s", *configPath)
	}

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化组件
	exec := executor.NewExecutor(cfg)

	// 初始化 Filter 和 Audit
	var filterInstance *filter.Filter
	if cfg.Rules.Masking != nil {
		filterInstance = filter.NewFilter(&cfg.Rules)
	}

	// 初始化审计日志记录器
	if cfg.Audit.LogFile != "" || cfg.Audit.AuditFile != "" {
		err = audit.Init(cfg.Audit)
		if err != nil {
			log.Printf("Failed to initialize audit logger: %v", err)
		}
	}

	handler := server.NewHandler(cfg, exec, filterInstance)
	srv := server.New(cfg, handler)

	// 启动服务器
	fmt.Println("Starting github.com/AceDarkknight/agent-kubectl-gateway...")
	if err := srv.Run(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
