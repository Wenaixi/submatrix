package utils

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Wenaixi/submatrix/config"
)

// ExecuteCallback 执行回调脚本
func ExecuteCallback(successCount int) {
	callbackScript := config.GlobalConfig.CallbackScript
	if callbackScript == "" {
		return
	}

	// 【安全修复 2026-11-10】回调脚本白名单限制：只允许执行 output/ 目录下的脚本。
	// 修掉「配置写 callback-script 指向任意路径 → 检测完成执行任意脚本」的 RCE 链（CRITICAL）。
	// 允许根 = <程序运行目录>/output，同时兼容绝对路径下的 output 目录。
	allowedRoot := filepath.Join("output")
	absScript, err := filepath.Abs(callbackScript)
	if err != nil {
		slog.Error(fmt.Sprintf("回调脚本路径解析失败: %s", callbackScript))
		return
	}
	absRoot, err := filepath.Abs(allowedRoot)
	if err != nil {
		slog.Error(fmt.Sprintf("回调脚本允许根解析失败: %s", allowedRoot))
		return
	}
	// 要求脚本真实路径必须位于 allowedRoot 之内（前缀判断 + 边界分隔符，防 /output-evil 绕过）
	if absScript != absRoot && !strings.HasPrefix(absScript, absRoot+string(filepath.Separator)) {
		slog.Error(fmt.Sprintf("回调脚本不在允许目录(输出目录)内，已拒绝执行: %s", callbackScript))
		return
	}

	slog.Info("执行回调脚本", "path", callbackScript)

	// 检查脚本文件是否存在
	if _, err := os.Stat(callbackScript); os.IsNotExist(err) {
		slog.Error(fmt.Sprintf("回调脚本不存在: %s", callbackScript))
		return
	}

	// 在非Windows系统上检查并设置执行权限
	if runtime.GOOS != "windows" {
		err := os.Chmod(callbackScript, 0o755) // rwxr-xr-x 权限
		if err != nil {
			slog.Warn(fmt.Sprintf("设置脚本执行权限失败: %v", err))
		}

		// 检查脚本是否有shebang
		content, err := os.ReadFile(callbackScript)
		if err == nil && len(content) > 0 {
			hasShebang := false
			if len(content) >= 2 && content[0] == '#' && content[1] == '!' {
				hasShebang = true
			}

			if !hasShebang {
				slog.Warn("脚本缺少shebang行，请在脚本开头添加对应的：#!/bin/bash、#!/bin/sh、#!/usr/bin/env bash 等")
			}
		}
	}

	// 根据操作系统类型选择不同的执行方式
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows 系统
		switch ext := strings.ToLower(filepath.Ext(callbackScript)); ext {
		case ".bat", ".cmd":
			// 使用完整路径，并正确处理带空格的路径
			absPath, err := filepath.Abs(callbackScript)
			if err != nil {
				slog.Error("获取脚本绝对路径失败: " + err.Error())
				return
			}
			cmd = exec.Command("cmd", "/C", absPath)

		case ".ps1":
			// PowerShell 脚本
			absPath, err := filepath.Abs(callbackScript)
			if err != nil {
				slog.Error("获取脚本绝对路径失败: " + err.Error())
				return
			}
			// 使用 -ExecutionPolicy Bypass 绕过执行策略限制
			cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", absPath)

		default:
			cmd = exec.Command(callbackScript)
		}

		// 设置工作目录为脚本所在目录
		cmd.Dir = filepath.Dir(callbackScript)
	} else {
		// Unix/Linux/MacOS 系统
		cmd = exec.Command(callbackScript)
	}

	// 设置环境变量，传递成功节点数量
	cmd.Env = append(os.Environ(), fmt.Sprintf("SUCCESS_COUNT=%d", successCount))

	// 执行命令
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error(fmt.Sprintf("执行回调脚本失败: %v, 输出: %s", err, string(output)))
		return
	}
	slog.Info("回调脚本执行成功")
}
