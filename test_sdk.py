#!/usr/bin/env python3
"""
AgentFense SDK 测试脚本

这个脚本演示了 Python SDK 的主要功能:
- 快速开始 (from_local)
- 权限控制测试
- Session 功能
- 文件操作
- 错误处理
- 预设 (presets) 使用

使用前请确保:
1. 安装 SDK: cd sdk/python && pip install -e .
2. 启动服务器: ./bin/agentfense-server -config test-config.yaml
"""

import sys
import os
from pathlib import Path

try:
    from agentfense import (
        Sandbox,
        SandboxClient,
        RuntimeType,
        ResourceLimits,
        list_presets,
        extend_preset,
        CommandTimeoutError,
        CommandExecutionError,
        PermissionDeniedError,
        PRESET_VIEW_ONLY,
        PRESET_READ_ONLY,
    )
except ImportError:
    print("❌ 无法导入 agentfense")
    print("请先安装 SDK: cd sdk/python && pip install -e .")
    sys.exit(1)


def print_section(title):
    """打印分节标题"""
    print("\n" + "=" * 70)
    print(f"  {title}")
    print("=" * 70)


def test_1_quick_start():
    """测试 1: 快速开始 - 使用 from_local 创建沙盒"""
    print_section("测试 1: 快速开始 (from_local)")
    
    # 创建一个临时测试目录
    test_dir = Path("./test_workspace")
    test_dir.mkdir(exist_ok=True)
    
    # 创建一些测试文件
    (test_dir / "hello.py").write_text('print("Hello from Sandbox!")')
    (test_dir / "README.md").write_text("# Test Project\n\nThis is a test.")
    
    try:
        # 一行代码创建沙盒并运行命令
        print("\n📦 创建沙盒并运行命令...")
        with Sandbox.from_local(str(test_dir)) as sandbox:
            print(f"✅ 沙盒已创建: {sandbox.id}")
            print(f"   运行时: {sandbox.runtime.value}")
            print(f"   状态: {sandbox.status.value}")
            
            # 运行简单命令
            result = sandbox.run("ls -la /workspace")
            print(f"\n📄 列出文件:")
            print(result.stdout)
            
            # 运行 Python 脚本
            result = sandbox.run("python /workspace/hello.py")
            print(f"🐍 运行 Python 脚本:")
            print(f"   输出: {result.stdout.strip()}")
            print(f"   退出码: {result.exit_code}")
            
        print("✅ 测试 1 完成 (沙盒已自动清理)")
        
    finally:
        # 清理测试目录
        import shutil
        if test_dir.exists():
            shutil.rmtree(test_dir)


def test_2_permissions():
    """测试 2: 四种权限级别 (none/view/read/write)"""
    print_section("测试 2: 权限控制")
    
    # 创建测试目录和文件
    test_dir = Path("./test_permissions")
    test_dir.mkdir(exist_ok=True)
    
    (test_dir / "public").mkdir(exist_ok=True)
    (test_dir / "public" / "readme.txt").write_text("Public file - readable")
    
    (test_dir / "docs").mkdir(exist_ok=True)
    (test_dir / "docs" / "guide.txt").write_text("Documentation - writable")
    
    (test_dir / "metadata").mkdir(exist_ok=True)
    (test_dir / "metadata" / "info.txt").write_text("Metadata - view only")
    
    (test_dir / "secrets").mkdir(exist_ok=True)
    (test_dir / "secrets" / ".env").write_text("DB_PASSWORD=secret123")
    
    try:
        print("\n📦 创建带自定义权限的沙盒...")
        # 注意：使用 PRESET_READ_ONLY 作为基础（默认所有文件可读），
        # 然后添加特定目录的覆盖规则。
        # 不要同时使用 preset 和相同 pattern 的自定义规则，会导致优先级冲突。
        with Sandbox.from_local(
            str(test_dir),
            preset=PRESET_READ_ONLY,  # 基础：所有文件可读
            permissions=[
                # /docs: 可写 (覆盖默认 read)
                {"pattern": "/docs/**", "permission": "write"},
                # /metadata: 只能列出文件名 (覆盖默认 read)
                {"pattern": "/metadata/**", "permission": "view"},
                # /secrets: 完全隐藏 (覆盖默认 read)
                {"pattern": "/secrets/**", "permission": "none"},
            ],
        ) as sandbox:
            print(f"✅ 沙盒已创建: {sandbox.id}\n")

            # 测试 read 权限
            print("🔍 测试 READ 权限 (/public):")
            result = sandbox.run("cat /workspace/public/readme.txt")
            print(f"   ✅ 可以读取文件: {result.stdout.strip()}")
            
            result = sandbox.run("echo 'modified' > /workspace/public/readme.txt || echo 'FAILED'")
            if "FAILED" in result.stdout or result.exit_code != 0:
                print(f"   ✅ 无法写入文件 (符合预期)")
            
            # 测试 write 权限
            print("\n✍️  测试 WRITE 权限 (/docs):")
            result = sandbox.run("echo 'new content' > /workspace/docs/new.txt && cat /workspace/docs/new.txt")
            print(f"   ✅ 可以创建和读取文件: {result.stdout.strip()}")
            
            # 测试 view 权限
            print("\n👁️  测试 VIEW 权限 (/metadata):")
            result = sandbox.run("ls /workspace/metadata")
            print(f"   ✅ 可以列出文件: {result.stdout.strip()}")
            
            result = sandbox.run("cat /workspace/metadata/info.txt 2>&1 || true")
            if "Permission denied" in result.stdout or result.exit_code != 0:
                print(f"   ✅ 无法读取文件内容 (符合预期)")
            
            # 测试 none 权限
            print("\n🚫 测试 NONE 权限 (/secrets):")
            result = sandbox.run("ls /workspace/secrets 2>&1 || echo 'NOT_FOUND'")
            if "NOT_FOUND" in result.stdout or "No such file" in result.stdout:
                print(f"   ✅ 目录完全隐藏 (符合预期)")
            
            result = sandbox.run("ls /workspace | grep secrets || echo 'NOT_VISIBLE'")
            if "NOT_VISIBLE" in result.stdout:
                print(f"   ✅ 在父目录中也不可见 (符合预期)")
        
        print("\n✅ 测试 2 完成")
        
    finally:
        import shutil
        if test_dir.exists():
            shutil.rmtree(test_dir)


def test_3_sessions():
    """测试 3: Session 功能 - 保持状态的 Shell 会话"""
    print_section("测试 3: Session 会话")
    
    test_dir = Path("./test_sessions")
    test_dir.mkdir(exist_ok=True)
    (test_dir / "script.sh").write_text("#!/bin/bash\necho 'Script executed!'")
    
    try:
        print("\n📦 创建沙盒...")
        with Sandbox.from_local(str(test_dir)) as sandbox:
            print(f"✅ 沙盒已创建: {sandbox.id}\n")
            
            # 使用 session 保持状态
            print("🔄 创建持久 Session:")
            with sandbox.session() as session:
                # 切换目录
                session.exec("cd /workspace")
                result = session.exec("pwd")
                print(f"   当前目录: {result.stdout.strip()}")
                
                # 设置环境变量
                session.exec("export MY_VAR='Hello Session'")
                result = session.exec("echo $MY_VAR")
                print(f"   环境变量: {result.stdout.strip()}")
                
                # 创建文件
                # 注意：默认 preset=agent-safe 时 /workspace 是只读的，可写目录为 /tmp 和 /output
                session.exec("echo 'persistent data' > /tmp/test.txt")
                result = session.exec("cat /tmp/test.txt")
                print(f"   文件内容: {result.stdout.strip()}")
                
            print("✅ Session 会话正常工作")
        
        print("\n✅ 测试 3 完成")
        
    finally:
        import shutil
        if test_dir.exists():
            shutil.rmtree(test_dir)


def test_4_presets():
    """测试 4: 预设 (Presets) 功能"""
    print_section("测试 4: 权限预设")
    
    # 列出所有可用预设
    print("\n📋 可用的权限预设:")
    presets = list_presets()
    for preset in presets:
        print(f"   - {preset}")
    
    test_dir = Path("./test_presets")
    test_dir.mkdir(exist_ok=True)
    (test_dir / "app.py").write_text("print('App running')")
    (test_dir / ".env").write_text("SECRET_KEY=super_secret")
    
    try:
        print("\n📦 使用 'agent-safe' 预设:")
        with Sandbox.from_local(
            str(test_dir),
            preset="agent-safe",  # 隐藏密钥文件
        ) as sandbox:
            # .env 应该被隐藏
            result = sandbox.run("ls -la /workspace | grep .env || echo 'NOT_FOUND'")
            if "NOT_FOUND" in result.stdout:
                print(f"   ✅ .env 文件被隐藏 (符合 agent-safe 预设)")
            
            # Python 文件应该可读
            result = sandbox.run("cat /workspace/app.py")
            if result.exit_code == 0:
                print(f"   ✅ .py 文件可读")
        
        print("\n📦 扩展预设:")
        # 在预设基础上添加自定义规则
        custom_rules = extend_preset(
            "agent-safe",
            additions=[
                {"pattern": "/custom/**", "permission": "write"}
            ]
        )
        print(f"   ✅ 成功扩展预设,共 {len(custom_rules)} 条规则")
        
        print("\n✅ 测试 4 完成")
        
    finally:
        import shutil
        if test_dir.exists():
            shutil.rmtree(test_dir)


def test_5_error_handling():
    """测试 5: 错误处理"""
    print_section("测试 5: 错误处理")
    
    test_dir = Path("./test_errors")
    test_dir.mkdir(exist_ok=True)
    (test_dir / "test.py").write_text("import sys; sys.exit(1)")
    
    try:
        print("\n📦 创建沙盒...")
        with Sandbox.from_local(str(test_dir)) as sandbox:
            
            # 测试命令超时
            print("\n⏱️  测试命令超时:")
            try:
                result = sandbox.run("sleep 5", timeout=2, raise_on_error=False)
                print("   ⚠️  命令未超时 (可能执行太快)")
            except CommandTimeoutError as e:
                print(f"   ✅ 正确捕获超时异常: {type(e).__name__}")
            
            # 测试命令失败
            print("\n❌ 测试命令执行失败:")
            try:
                result = sandbox.run("python /workspace/test.py", raise_on_error=True)
                print("   ⚠️  命令应该失败但没有抛出异常")
            except CommandExecutionError as e:
                print(f"   ✅ 正确捕获执行错误: {type(e).__name__}")
                print(f"   退出码: {e.exit_code}")
            
            # 测试不抛出异常的情况
            print("\n📊 测试不抛出异常模式:")
            result = sandbox.run("exit 42", raise_on_error=False)
            print(f"   ✅ 退出码: {result.exit_code} (不抛出异常)")
        
        print("\n✅ 测试 5 完成")
        
    finally:
        import shutil
        if test_dir.exists():
            shutil.rmtree(test_dir)


def test_6_file_operations():
    """测试 6: 文件操作"""
    print_section("测试 6: 文件操作")
    
    test_dir = Path("./test_files")
    test_dir.mkdir(exist_ok=True)
    (test_dir / "original.txt").write_text("Original content")
    
    try:
        print("\n📦 创建沙盒...")
        with Sandbox.from_local(str(test_dir), preset="full-access") as sandbox:
            
            # 读取文件
            print("\n📖 读取文件:")
            content = sandbox.read_file("/workspace/original.txt")
            print(f"   内容: {content}")
            
            # 写入文件
            print("\n✍️  写入文件:")
            sandbox.write_file("/workspace/new.txt", "New content from SDK")
            
            # 通过命令验证
            result = sandbox.run("cat /workspace/new.txt")
            print(f"   验证: {result.stdout.strip()}")
            
            # 列出文件
            print("\n📋 列出文件:")
            files = sandbox.list_files("/workspace")
            for f in files:
                print(f"   - {f}")
        
        print("\n✅ 测试 6 完成")
        
    finally:
        import shutil
        if test_dir.exists():
            shutil.rmtree(test_dir)


def test_7_low_level_api():
    """测试 7: 低级 API (SandboxClient)"""
    print_section("测试 7: 低级 API (SandboxClient)")
    
    print("\n🔌 连接到服务器...")
    client = SandboxClient(endpoint="localhost:9000")
    
    try:
        # 创建 codebase
        print("\n📦 创建 Codebase:")
        codebase = client.create_codebase(name="test-codebase", owner_id="test-user")
        print(f"   ✅ Codebase ID: {codebase.id}")
        
        # 上传文件
        print("\n⬆️  上传文件:")
        client.upload_file(codebase.id, "hello.py", b'print("Hello!")')
        print(f"   ✅ 文件已上传")
        
        # 创建沙盒
        print("\n🏗️  创建 Sandbox:")
        sandbox = client.create_sandbox(
            codebase_id=codebase.id,
            permissions=[
                {"pattern": "**/*", "permission": "read"},
            ],
        )
        print(f"   ✅ Sandbox ID: {sandbox.id}")
        
        # 启动沙盒
        print("\n▶️  启动 Sandbox:")
        client.start_sandbox(sandbox.id)
        print(f"   ✅ 沙盒已启动")
        
        # 执行命令
        print("\n⚙️  执行命令:")
        result = client.exec(sandbox.id, command="python /workspace/hello.py")
        print(f"   输出: {result.stdout.strip()}")
        print(f"   退出码: {result.exit_code}")
        
        # 清理
        print("\n🧹 清理资源:")
        client.destroy_sandbox(sandbox.id)
        print(f"   ✅ Sandbox 已销毁")
        
        client.delete_codebase(codebase.id)
        print(f"   ✅ Codebase 已删除")
        
        print("\n✅ 测试 7 完成")
        
    finally:
        client.close()


def test_8_docker_runtime():
    """测试 8: Docker Runtime 特性"""
    print_section("测试 8: Docker Runtime")
    
    test_dir = Path("./test_docker")
    test_dir.mkdir(exist_ok=True)
    (test_dir / "test.py").write_text("import sys; print(f'Python {sys.version}')")
    
    try:
        print("\n📦 创建使用 Docker 运行时的沙盒...")
        print("   镜像: python:3.11-alpine")
        print("   内存限制: 256MB")
        print("   进程限制: 50")
        
        with Sandbox.from_local(
            str(test_dir),
            runtime=RuntimeType.DOCKER,
            image="python:3.11-alpine",
            resources=ResourceLimits(
                memory_bytes=256 * 1024 * 1024,  # 256 MB
                pids_limit=50,
            ),
        ) as sandbox:
            print(f"\n✅ 沙盒已创建: {sandbox.id}")
            print(f"   运行时: {sandbox.runtime.value}")
            
            # 测试 Python 版本
            print("\n🐍 测试 Python 环境:")
            result = sandbox.run("python --version")
            print(f"   版本: {result.stdout.strip() or result.stderr.strip()}")
            
            # 运行脚本
            result = sandbox.run("python /workspace/test.py")
            print(f"   脚本输出: {result.stdout.strip()}")
            
            # 测试资源限制
            print("\n💾 测试内存限制:")
            result = sandbox.run("free -h || cat /proc/meminfo | head -3")
            print(f"   {result.stdout[:200]}")  # 只显示前 200 字符
            
        print("\n✅ 测试 8 完成")
        
    finally:
        import shutil
        if test_dir.exists():
            shutil.rmtree(test_dir)


def main():
    """运行所有测试"""
    print("""
╔══════════════════════════════════════════════════════════════════╗
║           AgentFense - Python SDK 测试脚本                       ║
║                  使用 Docker Runtime                              ║
╚══════════════════════════════════════════════════════════════════╝
    """)
    
    # 检查服务器是否运行
    print("🔍 检查服务器连接...")
    try:
        client = SandboxClient(endpoint="localhost:9000")
        client.close()
        print("✅ 服务器连接正常\n")
    except Exception as e:
        print(f"❌ 无法连接到服务器: {e}")
        print("\n请先启动服务器:")
        print("  ./bin/agentfense-server -config test-config.yaml")
        print("\n配置文件使用 Docker runtime，请确保:")
        print("  1. Docker 服务正在运行")
        print("  2. 当前用户有 Docker 权限")
        sys.exit(1)
    
    # 运行测试
    tests = [
        ("快速开始", test_1_quick_start),
        ("权限控制", test_2_permissions),
        ("Session 会话", test_3_sessions),
        ("权限预设", test_4_presets),
        ("错误处理", test_5_error_handling),
        ("文件操作", test_6_file_operations),
        ("低级 API", test_7_low_level_api),
        ("Docker Runtime", test_8_docker_runtime),
    ]
    
    passed = 0
    failed = 0
    
    for name, test_func in tests:
        try:
            test_func()
            passed += 1
        except KeyboardInterrupt:
            print("\n\n⚠️  测试被用户中断")
            sys.exit(1)
        except Exception as e:
            print(f"\n❌ 测试失败: {name}")
            print(f"   错误: {e}")
            import traceback
            traceback.print_exc()
            failed += 1
    
    # 打印总结
    print("\n" + "=" * 70)
    print("📊 测试总结")
    print("=" * 70)
    print(f"✅ 通过: {passed}")
    print(f"❌ 失败: {failed}")
    print(f"📈 总计: {passed + failed}")
    print("=" * 70)
    
    if failed > 0:
        sys.exit(1)
    else:
        print("\n🎉 所有测试通过!")


if __name__ == "__main__":
    main()
