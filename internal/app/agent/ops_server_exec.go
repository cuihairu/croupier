package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// execSensitiveEnvKeys 是「可改写白名单命令行为」的环境变量名集合（C-008）。
// LD_PRELOAD/LD_LIBRARY_PATH/LD_AUDIT 注入共享库；PATH 重定向二进制解析；
// PYTHON*/PERL*/RUBY*/NODE_* 注入解释器启动代码；BASH_ENV/ENV 注入 shell
// 启动文件；GIT_* 重定向 git 对象库；IFS/SHELLOPTS/BASHOPTS/CDPATH 改写
// shell 行为。拒绝任一命中（环境变量名惯例大写，比较做 upper 归一）。
var execSensitiveEnvKeys = map[string]struct{}{
	"LD_PRELOAD": {}, "LD_LIBRARY_PATH": {}, "LD_AUDIT": {}, "LD_DEBUG": {},
	"DYLD_INSERT_LIBRARIES": {}, "DYLD_LIBRARY_PATH": {},
	"PATH": {}, "CDPATH": {},
	"PYTHONPATH": {}, "PYTHONSTARTUP": {}, "PYTHONHOME": {}, "PYTHONINSPECT": {},
	"BASH_ENV": {}, "ENV": {}, "SHELLOPTS": {}, "BASHOPTS": {}, "IFS": {},
	"GIT_DIR": {}, "GIT_WORK_TREE": {}, "GIT_INDEX_FILE": {}, "GIT_OBJECT_DIRECTORY": {},
	"PERL5LIB": {}, "PERL5OPT": {},
	"NODE_OPTIONS": {},
	"RUBYOPT":      {}, "RUBYLIB": {},
	"RUSTC_BOOTSTRAP": {},
}

// execInlineCodeInterpreters 是「命令即解释器、参数即脚本」的二进制 basename
// （C-008）。白名单命中这些且携带内联/交互/特权参数（-c/-i/-p/-e）即任意
// 执行，命令白名单失去意义——须拒绝而非放行。
var execInlineCodeInterpreters = map[string]struct{}{
	"sh": {}, "bash": {}, "dash": {}, "zsh": {}, "ksh": {}, "ash": {},
	"fish": {}, "csh": {}, "tcsh": {},
	"cmd": {}, "cmd.exe": {}, "powershell": {}, "pwsh": {},
	"python": {}, "python2": {}, "python3": {},
	"perl": {}, "ruby": {}, "node": {}, "php": {}, "lua": {},
}

// execInlineCodeFlags 是上述解释器中把参数当内联代码执行的开关。
var execInlineCodeFlags = map[string]struct{}{
	"-c": {}, "-i": {}, "-p": {}, "-e": {}, "-r": {},
}

// validateExecBoundary 校验白名单命令的 Args/Env/WorkingDir 不用于改写行为。
// fail-closed：任一命中即返回错误，调用方不得执行。
func validateExecBoundary(req *opsv1.ExecuteCommandRequest) error {
	// 1) 环境变量：敏感键拒绝（LD_PRELOAD/PATH 等）。
	for k := range req.Env {
		upper := strings.ToUpper(k)
		if _, bad := execSensitiveEnvKeys[upper]; bad {
			return fmt.Errorf("exec env variable '%s' is not allowed (sensitive key)", k)
		}
	}

	// 2) 参数：解释器 + 内联代码开关 = 任意执行，拒绝。
	base := filepath.Base(req.Command)
	if _, isInterp := execInlineCodeInterpreters[base]; isInterp {
		for _, a := range req.Args {
			if _, bad := execInlineCodeFlags[a]; bad {
				return fmt.Errorf("exec command '%s' is an interpreter: arg '%s' allows arbitrary execution", base, a)
			}
		}
	}

	// 3) 工作目录：必须绝对路径且原始路径不含 .. 段（Clean 会把 .. 解析掉，
	// 须在清洗前检查原始段，否则 /tmp/../../etc 会静默变 /etc 逃逸）。
	if req.WorkingDir != "" {
		if !filepath.IsAbs(req.WorkingDir) {
			return fmt.Errorf("exec working dir must be absolute")
		}
		for _, seg := range strings.Split(req.WorkingDir, string(os.PathSeparator)) {
			if seg == ".." {
				return fmt.Errorf("exec working dir must not contain '..'")
			}
		}
	}

	return nil
}

// ExecuteCommand executes a command on the agent.
func (s *OpsServer) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	if !s.config.Enabled || !s.config.AllowExec {
		return nil, fmt.Errorf("ops exec is not enabled")
	}

	// 白名单前置（§10.7 远程 Shell 边界）：命中才放行；空列表一律拒绝
	// （fail-closed）——Validate 已拦配置面，此处兜底程序内直构路径。
	allowed := false
	for _, cmd := range s.config.ExecAllowedCommands {
		if cmd == req.Command {
			allowed = true
			break
		}
	}
	if !allowed {
		if len(s.config.ExecAllowedCommands) == 0 {
			return nil, fmt.Errorf("exec whitelist is empty: allowExec requires execAllowedCommands")
		}
		return nil, fmt.Errorf("command '%s' is not allowed", req.Command)
	}

	// C-008：白名单只锁命令名。Args/Env/WorkingDir 原样透传会让「允许 ls」
	// 变成任意行为（LD_PRELOAD 注入共享库、PATH 重定向二进制、shell -c 任意
	// 命令、WorkingDir 逃逸）。三层校验均 fail-closed，命中即拒。
	if err := validateExecBoundary(req); err != nil {
		return nil, err
	}

	// Create command with timeout
	timeout := s.config.ExecTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}
	if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// Set environment variables if provided
	if len(req.Env) > 0 {
		env := os.Environ()
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	output, err := cmd.CombinedOutput()

	exitCode := int32(0)
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitError.ExitCode())
		} else {
			exitCode = -1
		}
	}

	return &opsv1.ExecuteCommandResponse{
		ExitCode: exitCode,
		StdOut:   string(output),
		StdErr:   "", // Combined in stdout
	}, nil
}
