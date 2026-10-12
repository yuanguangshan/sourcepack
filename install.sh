#!/bin/bash

# Sourcepack (gdoc) 安装脚本

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="sourcepack"
SHORTCUT_NAMES=("gdoc" "gdx" "gx" "doc")
SOURCE_FILE="go.mod"   # 入口是包（main.go + changeset.go + changeoutput.go）

# 检查权限
USE_SUDO=""
if [ ! -w "$INSTALL_DIR" ]; then
    USE_SUDO="sudo"
fi

# 卸载逻辑
if [[ "$1" == "--uninstall" ]]; then
    echo "🗑 卸载 Sourcepack..."
    for name in "${SHORTCUT_NAMES[@]}"; do
        $USE_SUDO rm -f "$INSTALL_DIR/$name"
    done

    # 清理旧版遗留的 gd / godoc 快捷方式
    $USE_SUDO rm -f "$INSTALL_DIR/gd" "$INSTALL_DIR/godoc"

    # 额外检查 ~/.local/bin 以防万一（这里原来写的是 $SHORTCUT_NAME，
    # 而变量名是数组 SHORTCUT_NAMES，于是这个分支从来没执行过）
    for name in "${SHORTCUT_NAMES[@]}"; do
        if [[ -f "$HOME/.local/bin/$name" ]]; then
            rm -f "$HOME/.local/bin/$name"
        fi
    done

    echo "✓ 已成功卸载"
    exit 0
fi

echo "🚀 开始安装 Sourcepack..."

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
echo "📦 编译 Sourcepack..."
go build -o "$BINARY_NAME" .
if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi

# 安装二进制
echo "📥 安装到 $INSTALL_DIR..."
$USE_SUDO mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
$USE_SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

for name in "${SHORTCUT_NAMES[@]}"; do
    echo "🔗 创建快捷命令 $name..."
    $USE_SUDO ln -sf "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR/$name"
done

# 清理旧版遗留的 gd / godoc 快捷方式
$USE_SUDO rm -f "$INSTALL_DIR/gd" "$INSTALL_DIR/godoc"

echo -e "\n✅ 安装完成！"
"$INSTALL_DIR/$BINARY_NAME" --version

echo -e "\n现在你可以运行："
echo "  sourcepack   # 主命令（与包名一致）"
echo "  gdoc / gdx   # 快捷命令"
