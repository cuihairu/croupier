package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var (
	serviceName        string
	serviceDisplayName string
	serviceDescription string
	serviceConfigDir   string
)

func init() {
	// 默认服务配置
	serviceName = "CroupierAgent"
	serviceDisplayName = "Croupier Agent - Game Operations Control Plane Agent"
	serviceDescription = "Croupier Agent 用于与游戏实例交互、分发作业以及上报指标"
	serviceConfigDir = defaultConfigDir()

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
		RunE:  runServiceInstall,
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
		RunE:  runServiceUninstall,
	}

	// 启动命令
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "启动服务",
		Long:  `启动已安装的系统服务`,
		RunE:  runServiceStart,
	}

	// 停止命令
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "停止服务",
		Long:  `停止正在运行的服务`,
		RunE:  runServiceStop,
	}

	// 重启命令
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "重启服务",
		Long:  `重启正在运行的服务`,
		RunE:  runServiceRestart,
	}

	// 状态命令
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "查看服务状态",
		Long:  `显示服务的当前运行状态`,
		RunE:  runServiceStatus,
	}

	// 运行命令 (由服务管理器调用)
	runCmd := &cobra.Command{
		Use:    "run",
		Short:  "运行服务 (由系统服务管理器调用)",
		Hidden: true,
		RunE:   runServiceRun,
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

// croupierAgentService 实现 service.Interface
type croupierAgentService struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cfgFile string
}

func newAgentService(cfgFile string) *croupierAgentService {
	ctx, cancel := context.WithCancel(context.Background())
	return &croupierAgentService{
		ctx:     ctx,
		cancel:  cancel,
		cfgFile: cfgFile,
	}
}

// Start 由服务管理器调用
func (s *croupierAgentService) Start(svc service.Service) error {
	// 设置全局配置文件
	if s.cfgFile != "" {
		cfgFile = s.cfgFile
	}

	s.logger("info", "Croupier Agent 服务启动中...")

	// 启动 agent
	go func() {
		if err := runAgent(); err != nil {
			s.logger("error", fmt.Sprintf("Agent 启动失败: %v", err))
			// 启动失败，停止服务
			svc.Stop()
		} else {
			s.logger("info", "Croupier Agent 服务已启动")
		}
	}()

	// 等待上下文取消
	go func() {
		<-s.ctx.Done()
		svc.Stop()
	}()

	return nil
}

// Stop 由服务管理器调用
func (s *croupierAgentService) Stop(svc service.Service) error {
	s.logger("info", "Croupier Agent 服务停止中...")
	s.cancel()
	return nil
}

func (s *croupierAgentService) logger(level, msg string) {
	// 简单的日志输出，因为主日志系统可能还没初始化
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", timestamp, level, msg)
}

// 创建服务对象
func createService() (service.Service, error) {
	// 确定可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("无法获取可执行文件路径: %w", err)
	}

	// 确定配置文件路径
	configPath := serviceConfigDir
	if configPath == "" {
		configPath = defaultConfigDir()
	}
	if !filepath.IsAbs(configPath) {
		// 如果是相对路径，转换为绝对路径
		configPath, err = filepath.Abs(configPath)
		if err != nil {
			returnPath, err := filepath.Abs(execPath)
			if err != nil {
				return nil, fmt.Errorf("无法转换配置路径: %w", err)
			}
			configPath = filepath.Join(filepath.Dir(returnPath), configPath)
		}
	}

	// 配置文件路径
	cfgFile := filepath.Join(configPath, "agent.yaml")
	if cfgFileFlag := getFlagValue("config"); cfgFileFlag != "" {
		cfgFile = cfgFileFlag
	}

	// 服务参数: 使用 "run" 子命令
	svcConfig := &service.Config{
		Name:        serviceName,
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		Executable:  execPath,
		Arguments:   []string{"--config", cfgFile, "service", "run"},
	}

	// 平台特定配置
	if service.Platform() == "windows" {
		// Windows: 设置为自动启动
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

	// 创建服务对象
	svc, err := service.New(nil, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("创建服务对象失败: %w", err)
	}

	return svc, nil
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	serviceName, _ = cmd.Flags().GetString("name")
	serviceDisplayName, _ = cmd.Flags().GetString("display-name")
	serviceDescription, _ = cmd.Flags().GetString("description")
	serviceConfigDir, _ = cmd.Flags().GetString("config-dir")

	svc, err := createService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	// 检查服务是否已存在
	status, err := svc.Status()
	if err == nil && status != service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 已存在 (状态: %d)", serviceName, status)
	}

	// 安装服务
	fmt.Printf("正在安装服务 '%s'...\n", serviceName)
	fmt.Printf("  可执行文件: %s\n", svc.Platform()[:1])
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

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	svc, err := createService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	// 检查服务状态
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("查询服务状态失败: %w", err)
	}

	if status == service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 不存在", serviceName)
	}

	// 如果服务正在运行，先停止
	if status == service.StatusRunning {
		fmt.Printf("正在停止服务 '%s'...\n", serviceName)
		if err := svc.Stop(); err != nil {
			return fmt.Errorf("停止服务失败: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	// 卸载服务
	fmt.Printf("正在卸载服务 '%s'...\n", serviceName)
	if err := svc.Uninstall(); err != nil {
		return fmt.Errorf("卸载服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已卸载\n", serviceName)
	return nil
}

func runServiceStart(cmd *cobra.Command, args []string) error {
	svc, err := createService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	// 检查服务状态
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

	// 启动服务
	fmt.Printf("正在启动服务 '%s'...\n", serviceName)
	if err := svc.Start(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已启动\n", serviceName)
	return nil
}

func runServiceStop(cmd *cobra.Command, args []string) error {
	svc, err := createService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	// 检查服务状态
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

	// 停止服务
	fmt.Printf("正在停止服务 '%s'...\n", serviceName)
	if err := svc.Stop(); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已停止\n", serviceName)
	return nil
}

func runServiceRestart(cmd *cobra.Command, args []string) error {
	svc, err := createService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	// 检查服务状态
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("查询服务状态失败: %w", err)
	}

	if status == service.StatusUnknown {
		return fmt.Errorf("服务 '%s' 不存在", serviceName)
	}

	fmt.Printf("正在重启服务 '%s'...\n", serviceName)

	// 先停止
	if status == service.StatusRunning {
		if err := svc.Stop(); err != nil {
			return fmt.Errorf("停止服务失败: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	// 再启动
	if err := svc.Start(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	fmt.Printf("✅ 服务 '%s' 已重启\n", serviceName)
	return nil
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	svc, err := createService()
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	// 获取服务状态
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

	// 获取平台特定信息
	fmt.Printf("平台: %s\n", svc.Platform())

	// Windows 额外信息
	if svc.Platform() == "windows" {
		if status == service.StatusRunning {
			fmt.Printf("\n管理命令:\n")
			fmt.Printf("  PowerShell: Get-Service %s\n", serviceName)
			fmt.Printf("  控制面板: services.msc\n")
		}
	}

	// Linux 额外信息
	if svc.Platform() == "linux" {
		fmt.Printf("\n管理命令:\n")
		fmt.Printf("  systemctl status %s\n", serviceName)
		fmt.Printf("  journalctl -u %s -f\n", serviceName)
	}

	return nil
}

func runServiceRun(cmd *cobra.Command, args []string) error {
	// 确定配置文件路径
	cfgFilePath := serviceConfigDir
	if cfgFilePath == "" {
		cfgFilePath = defaultConfigDir()
	}
	cfgFile = filepath.Join(cfgFilePath, "agent.yaml")

	// 检查配置文件是否存在
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s\n请先创建配置文件或安装服务", cfgFile)
	}

	// 创建服务对象
	svcObj := newAgentService(cfgFile)

	// 使用 kardianos/service 运行
	// 这会阻塞直到服务停止
	s, err := createService()
	if err != nil {
		return fmt.Errorf("创建服务对象失败: %w", err)
	}

	// 使用 kardianos/service 的控制机制
	errChan := make(chan error, 1)

	go func() {
		errChan <- svcObj.Start(s)
	}()

	// 等待系统停止信号
	return <-errChan
}

// defaultConfigDir 返回默认配置目录
// 优先级: 环境变量 > 可执行文件目录/etc > 系统配置目录
func defaultConfigDir() string {
	// 1. 环境变量优先
	if dir := os.Getenv("CROUPIER_CONFIG_DIR"); dir != "" {
		return dir
	}

	// 2. 使用可执行文件所在目录下的 etc 目录
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		etcDir := filepath.Join(execDir, "etc")
		// 检查 etc 目录是否存在
		if info, err := os.Stat(etcDir); err == nil && info.IsDir() {
			return etcDir
		}
	}

	// 3. 回退到系统配置目录
	switch service.Platform() {
	case "windows":
		return "C:\\ProgramData\\Croupier\\config"
	case "darwin":
		return "/etc/croupier"
	default: // linux
		return "/etc/croupier"
	}
}

// getFlagValue 获取命令行标志值
func getFlagValue(name string) string {
	// 从命令行参数中解析
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
