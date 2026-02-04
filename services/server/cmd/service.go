// Package cmd implements system service management commands for croupier-server
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
)

var (
	serviceName        string
	serviceDisplayName string
	serviceDescription string
	serviceConfigDir   string
)

func initServiceCommand() {
	// 默认服务配置
	serviceName = "CroupierServer"
	serviceDisplayName = "Croupier Server - Game Operations Control Plane"
	serviceDescription = "Croupier 游戏管理服务器 - 三层分布式游戏管理后端系统"
	serviceConfigDir = defaultServerConfigDir()

	// 服务命令
	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "系统服务管理",
		Long:  `管理系统服务的安装、卸载、启动、停止等操作`,
	}

	// 安装命令
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "安装为系统服务",
		Long:  `将当前程序安装为系统服务，支持开机自启动`,
		RunE:  runServerServiceInstall,
	}
	installCmd.Flags().StringVar(&serviceName, "name", serviceName, "服务名称")
	installCmd.Flags().StringVar(&serviceDisplayName, "display-name", serviceDisplayName, "服务显示名称")
	installCmd.Flags().StringVar(&serviceDescription, "description", serviceDescription, "服务描述")
	installCmd.Flags().StringVar(&serviceConfigDir, "config-dir", serviceConfigDir, "配置文件目录")

	// 卸载命令
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "卸载系统服务",
		Long:  `从系统中卸载已安装的服务`,
		RunE:  runServerServiceUninstall,
	}

	// 启动命令
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "启动服务",
		Long:  `启动已安装的系统服务`,
		RunE:  runServerServiceStart,
	}

	// 停止命令
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "停止服务",
		Long:  `停止正在运行的服务`,
		RunE:  runServerServiceStop,
	}

	// 重启命令
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "重启服务",
		Long:  `重启正在运行的服务`,
		RunE:  runServerServiceRestart,
	}

	// 状态命令
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "查看服务状态",
		Long:  `显示服务的当前运行状态`,
		RunE:  runServerServiceStatus,
	}

	// 运行命令 (由服务管理器调用)
	runCmd := &cobra.Command{
		Use:    "run",
		Short:  "运行服务 (由系统服务管理器调用)",
		Hidden: true,
		RunE:   runServerServiceRun,
	}

	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
	serviceCmd.AddCommand(startCmd)
	serviceCmd.AddCommand(stopCmd)
	serviceCmd.AddCommand(restartCmd)
	serviceCmd.AddCommand(statusCmd)
	serviceCmd.AddCommand(runCmd)

	rootCmd.AddCommand(serviceCmd)
}

// croupierServerService 实现 service.Interface
type croupierServerService struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cfgFile string
}

func newServerService(cfgFile string) *croupierServerService {
	ctx, cancel := context.WithCancel(context.Background())
	return &croupierServerService{
		ctx:     ctx,
		cancel:  cancel,
		cfgFile: cfgFile,
	}
}

// Start 由服务管理器调用
func (s *croupierServerService) Start(svc service.Service) error {
	// 从配置文件初始化日志系统
	s.initLoggingFromConfig()

	slog.Info("=== Croupier Server 服务启动 ===",
		"config_file", s.cfgFile,
		"working_dir", wd(),
		"executable", exePath())

	// 检查配置文件是否存在
	if s.cfgFile != "" {
		if _, err := os.Stat(s.cfgFile); os.IsNotExist(err) {
			slog.Error("配置文件不存在", "file", s.cfgFile)
			return fmt.Errorf("配置文件不存在: %s", s.cfgFile)
		}
		// 设置全局配置文件
		cfgFile = s.cfgFile
	}

	// 启动 server
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Server panic", "error", r)
				svc.Stop()
			}
		}()

		slog.Info("正在调用 runServer()...")
		if err := runServer(); err != nil {
			slog.Error("Server 启动失败", "error", err)
			// 启动失败，停止服务
			svc.Stop()
		} else {
			slog.Info("Croupier Server 服务已启动")
		}
	}()

	// 等待上下文取消
	go func() {
		<-s.ctx.Done()
		slog.Info("收到停止信号")
		svc.Stop()
	}()

	return nil
}

// Stop 由服务管理器调用
func (s *croupierServerService) Stop(svc service.Service) error {
	slog.Info("Croupier Server 服务停止中...")
	s.cancel()
	return nil
}

// initLoggingFromConfig 从配置文件初始化日志系统
func (s *croupierServerService) initLoggingFromConfig() {
	if s.cfgFile == "" {
		return
	}

	var cfg config.Config
	if err := conf.LoadConfig(s.cfgFile, &cfg); err != nil {
		// 配置加载失败，使用默认日志配置
		common.SetupLoggerWithFile("info", "console", "", 0, 0, 0, false)
		return
	}

	// 使用配置中的日志设置
	logCfg := cfg.Log
	if logCfg.Level == "" {
		logCfg.Level = "info"
	}
	if logCfg.Format == "" {
		logCfg.Format = "console"
	}
	common.SetupLoggerWithFile(
		logCfg.Level,
		logCfg.Format,
		logCfg.File,
		logCfg.MaxSize,
		logCfg.MaxBackups,
		logCfg.MaxAge,
		logCfg.Compress,
	)
}

// 创建服务对象
func createServerService() (service.Service, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("无法获取可执行文件路径: %w", err)
	}

	configPath := serviceConfigDir
	if configPath == "" {
		configPath = defaultServerConfigDir()
	}
	if !filepath.IsAbs(configPath) {
		configPath, err = filepath.Abs(configPath)
		if err != nil {
			returnPath, err := filepath.Abs(execPath)
			if err != nil {
				return nil, fmt.Errorf("无法转换配置路径: %w", err)
			}
			configPath = filepath.Join(filepath.Dir(returnPath), configPath)
		}
	}

	cfgFile := filepath.Join(configPath, "server.yaml")
	if cfgFileFlag := getServerFlagValue("config"); cfgFileFlag != "" {
		cfgFile = cfgFileFlag
	}

	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		Executable:  execPath,
		Arguments:   []string{"--config", cfgFile, "service", "run"},
	}

	if service.Platform() == "windows" {
		svcConfig.Option = service.KeyValue{
			"StartType": "auto",
		}
	}

	if service.Platform() == "linux" {
		svcConfig.Dependencies = []string{
			"After=network-online.target",
			"Wants=network-online.target",
		}
		svcConfig.UserName = "croupier"
	}

	svc, err := service.New(nil, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("创建服务对象失败: %w", err)
	}

	return svc, nil
}

func runServerServiceInstall(cmd *cobra.Command, args []string) error {
	serviceName, _ = cmd.Flags().GetString("name")
	serviceDisplayName, _ = cmd.Flags().GetString("display-name")
	serviceDescription, _ = cmd.Flags().GetString("description")
	serviceConfigDir, _ = cmd.Flags().GetString("config-dir")

	svc, err := createServerService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	status, err := svc.Status()
	if err == nil && status != service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 已存在 (状态: %d)", serviceName, status)
	}

	fmt.Printf("正在安装服务 '%s'...\n", serviceName)
	fmt.Printf("  平台: %s\n", svc.Platform())
	fmt.Printf("  配置目录: %s\n", serviceConfigDir)

	if err := svc.Install(); err != nil {
		return fmt.Errorf("安装服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 安装成功\n", serviceName)
	fmt.Println("\n后续操作:")
	fmt.Printf("  启动服务:   %s service start\n", rootCmd.Name())
	fmt.Printf("  查看状态:   %s service status\n", rootCmd.Name())
	fmt.Printf("  停止服务:   %s service stop\n", rootCmd.Name())
	fmt.Printf("  卸载服务:   %s service uninstall\n", rootCmd.Name())

	return nil
}

func runServerServiceUninstall(cmd *cobra.Command, args []string) error {
	svc, err := createServerService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("查询服务状态失败: %w", err)
	}

	if status == service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 不存在", serviceName)
	}

	if status == service.StatusRunning {
		fmt.Printf("正在停止服务 '%s'...\n", serviceName)
		if err := svc.Stop(); err != nil {
			return fmt.Errorf("停止服务失败: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Printf("正在卸载服务 '%s'...\n", serviceName)
	if err := svc.Uninstall(); err != nil {
		return fmt.Errorf("卸载服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已卸载\n", serviceName)
	return nil
}

func runServerServiceStart(cmd *cobra.Command, args []string) error {
	svc, err := createServerService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("查询服务状态失败: %w", err)
	}

	if status == service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 不存在，请先执行: %s service install", serviceName, rootCmd.Name())
	}

	if status == service.StatusRunning {
		fmt.Printf("服务 '%s' 已在运行中\n", serviceName)
		return nil
	}

	fmt.Printf("正在启动服务 '%s'...\n", serviceName)
	if err := svc.Start(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已启动\n", serviceName)
	return nil
}

func runServerServiceStop(cmd *cobra.Command, args []string) error {
	svc, err := createServerService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("查询服务状态失败: %w", err)
	}

	if status == service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 不存在", serviceName)
	}

	if status == service.StatusStopped {
		fmt.Printf("服务 '%s' 已停止\n", serviceName)
		return nil
	}

	fmt.Printf("正在停止服务 '%s'...\n", serviceName)
	if err := svc.Stop(); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已停止\n", serviceName)
	return nil
}

func runServerServiceRestart(cmd *cobra.Command, args []string) error {
	svc, err := createServerService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("查询服务状态失败: %w", err)
	}

	if status == service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 不存在", serviceName)
	}

	fmt.Printf("正在重启服务 '%s'...\n", serviceName)

	if status == service.StatusRunning {
		if err := svc.Stop(); err != nil {
			return fmt.Errorf("停止服务失败: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	if err := svc.Start(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已重启\n", serviceName)
	return nil
}

func runServerServiceStatus(cmd *cobra.Command, args []string) error {
	svc, err := createServerService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("查询服务状态失败: %w", err)
	}

	fmt.Printf("服务名称: %s\n", serviceName)
	fmt.Printf("显示名称: %s\n", serviceDisplayName)
	fmt.Printf("状态: ")

	switch status {
	case service.StatusRunning:
		fmt.Println("运行中 ✓")
	case service.StatusStopped:
		fmt.Println("已停止")
	case service.StatusUnknown:
		fmt.Println("未安装")
		fmt.Printf("\n请先执行: %s service install\n", rootCmd.Name())
		return nil
	}

	fmt.Printf("平台: %s\n", svc.Platform())

	if svc.Platform() == "windows" {
		if status == service.StatusRunning {
			fmt.Printf("\n管理命令:\n")
			fmt.Printf("  PowerShell: Get-Service %s\n", serviceName)
			fmt.Printf("  控制面板: services.msc\n")
		}
	}

	if svc.Platform() == "linux" {
		fmt.Printf("\n管理命令:\n")
		fmt.Printf("  systemctl status %s\n", serviceName)
		fmt.Printf("  journalctl -u %s -f\n", serviceName)
	}

	return nil
}

func runServerServiceRun(cmd *cobra.Command, args []string) error {
	cfgFilePath := serviceConfigDir
	if cfgFilePath == "" {
		cfgFilePath = defaultServerConfigDir()
	}
	cfgFile = filepath.Join(cfgFilePath, "server.yaml")

	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s\n请先创建配置文件或安装服务", cfgFile)
	}

	svcObj := newServerService(cfgFile)
	s, err := createServerService()
	if err != nil {
		return fmt.Errorf("创建服务对象失败: %w", err)
	}

	errChan := make(chan error, 1)

	go func() {
		errChan <- svcObj.Start(s)
	}()

	return <-errChan
}

// serverServiceWrapper 适配 kardianos/service 接口
type serverServiceWrapper struct {
	service.Interface
	svc service.Service
}

// defaultServerConfigDir 返回默认配置目录
func defaultServerConfigDir() string {
	if dir := os.Getenv("CROUPIER_CONFIG_DIR"); dir != "" {
		return dir
	}

	switch service.Platform() {
	case "windows":
		return "C:\\ProgramData\\Croupier\\config"
	case "darwin":
		return "/etc/croupier"
	default:
		return "/etc/croupier"
	}
}

// getServerFlagValue 获取命令行标志值
func getServerFlagValue(name string) string {
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--"+name || arg == "-"+name[:1] {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return args[i+1]
			}
		}
		if strings.HasPrefix(arg, "--"+name+"=") {
			_, value, _ := strings.Cut(arg, "=")
			return value
		}
	}
	return ""
}

// wd 返回当前工作目录
func wd() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "unknown"
}

// exePath 返回可执行文件路径
func exePath() string {
	if path, err := os.Executable(); err == nil {
		return path
	}
	return "unknown"
}
