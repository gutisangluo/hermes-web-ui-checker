@echo off
title Hermes Web UI 检测修复工具
chcp 65001 >nul

echo 正在启动检查工具...
echo 如果系统询问执行策略，请选择 Y(是)，或在 PowerShell 中先运行:
echo Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0check-hermes-web-ui.ps1"

if %errorlevel% neq 0 (
    echo.
    echo 脚本执行出错。如果是因为执行策略限制，请尝试:
    echo 1. 以管理员身份运行 PowerShell
    echo 2. 执行: Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
    echo 3. 然后重新双击此文件
    pause
)
