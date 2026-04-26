#!/bin/bash

# Godoc (gd) 安装脚本

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="godoc"
SHORTCUT_NAME="gd"
SOURCE_FILE="godoc.go"

# 检查权限
USE_SUDO=""
if [ ! -w "$INSTALL_DIR" ]; then
    USE_SUDO="sudo"
fi

# 卸载逻辑
if [[ "$1" == "--uninstall" ]]; then
    echo "🗑 卸载 Godoc..."
    $USE_SUDO rm -f "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR/$SHORTCUT_NAME"
    
    # 额外检查 ~/.local/bin 以防万一
    if [[ -f "$HOME/.local/bin/$SHORTCUT_NAME" ]]; then
        rm -f "$HOME/.local/bin/$SHORTCUT_NAME"
    fi
    
    echo "✓ 已成功卸载"
    exit 0
fi

echo "🚀 开始安装 Godoc..."

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未找到 Go 环境，请先安装 Go"
    exit 1
fi

# 检查源文件
if [[ ! -f "$SOURCE_FILE" ]]; then
    echo "❌ 错误: 未找到 $SOURCE_FILE"
    exit 1
fi

# 编译
echo "📦 编译 Godoc..."
go build -o "$BINARY_NAME" "$SOURCE_FILE"
if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi

# 安装二进制
echo "📥 安装到 $INSTALL_DIR..."
$USE_SUDO mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
$USE_SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

# 创建/更新快捷命令
echo "🔗 创建快捷命令 $SHORTCUT_NAME..."
$USE_SUDO ln -sf "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR/$SHORTCUT_NAME"

# 检查是否由于 PATH 优先级导致旧版本干扰
EXISTING_GD=$(which $SHORTCUT_NAME)
if [[ "$EXISTING_GD" != "$INSTALL_DIR/$SHORTCUT_NAME" ]]; then
    echo "⚠️  注意: 发现另一个 $SHORTCUT_NAME 位于 $EXISTING_GD"
    echo "提示: 当前运行的可能是旧版本。建议删除旧版本或调整 PATH 顺序。"
fi

echo -e "\n✅ 安装完成！"
"$INSTALL_DIR/$BINARY_NAME" --version

echo -e "\n现在你可以运行："
echo "  godoc         # 完整命令"
echo "  gd           # 快捷命令"
