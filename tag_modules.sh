#!/bin/bash

# 获取项目根目录
root_dir=$(pwd)

# 函数：为指定模块创建标签
create_tag_for_module() {
    local module_dir=$1
    local module_name=$(basename "$module_dir")

    # 切换到模块目录
    cd "$module_dir"

    # 提示用户输入版本号
    read -p "Enter version for $module_name (e.g., 1.0.0): " version

    # 创建标签（使用轻量级标签）
    tag_name="${module_name}/v${version}"
    git tag "$tag_name"

    echo "Created tag $tag_name for $module_name"

    # 返回到根目录
    cd "$root_dir"
}

# 函数：显示特定模块的标签历史
show_module_tags() {
    local module_name=$1
    echo "Tags for module $module_name:"
    git tag -l "${module_name}/*" --sort=-v:refname
}

# 主菜单
while true; do
    echo "
1. Tag all modules
2. Tag a specific module
3. Show tags for a module
4. Push all tags to remote
5. Exit
"
    read -p "Choose an option: " choice

    case $choice in
        1)
            # 为所有模块创建标签
            for module_dir in $(find . -name go.mod -exec dirname {} \;)
            do
                create_tag_for_module "$module_dir"
            done
            ;;
        2)
            # 为特定模块创建标签
            read -p "Enter module name: " module_name
            if [ -d "$module_name" ] && [ -f "$module_name/go.mod" ]; then
                create_tag_for_module "$module_name"
            else
                echo "Module not found or invalid."
            fi
            ;;
        3)
            # 显示特定模块的标签
            read -p "Enter module name: " module_name
            show_module_tags "$module_name"
            ;;
        4)
            # 推送所有标签到远程仓库
            git push --tags
            echo "All tags have been pushed to the remote repository."
            ;;
        5)
            # 退出
            echo "Exiting."
            exit 0
            ;;
        *)
            echo "Invalid option. Please try again."
            ;;
    esac
done