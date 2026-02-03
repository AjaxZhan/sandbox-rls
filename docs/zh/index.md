# AgentFense

> 为 AI Agent 提供最小权限文件系统沙盒与上下文护栏

在真实代码库上运行不可信的 AI Agent 代码，同时在文件级别强制执行**最小权限访问**。

## 面临的挑战

最好的 agent 接口依然是简单的：**bash + 文件系统**。通过 FUSE，你可以挂载任何环境，让 agent 用简单的 `ls`、`cat`、`grep` 和 `find` 就能高效工作。

但存在一个问题：文件系统通常是**全有或全无**的。挂载一个真实仓库，往往会暴露*所有内容*——包括密钥。

## 解决方案

AgentFense 通过四级权限填补了这一空白：

| 级别 | Agent 能做什么 |
|-------|------------------------|
| `none`  | 路径**不可见**（`ls` 看不到，就像不存在一样） |
| `view`  | 可以**列出**名称（`ls`），但无法读取文件内容 |
| `read`  | 可以读取文件内容 |
| `write` | 可以读取 + 修改 / 创建文件 |

**示例策略**：*"你可以编辑 `/docs`，查看 `/metadata`，读取其他所有内容，但 `/secrets` 不存在。"*

## 快速示例

```python
from agentfense import Sandbox

# 一行代码：从本地目录创建沙盒，使用 "agent-safe" 预设
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
```

`agent-safe` 预设：读取所有文件，写入 `/output` 和 `/tmp`，隐藏密钥（`.env`、`*.key` 等）。

## 下一步

- [快速开始](get-started/quickstart.md) - 5 分钟上手
- [核心概念](get-started/concepts.md) - 理解基本原理
- [为什么选择 AgentFense](why/index.md) - 了解用例和优势
- [Python SDK](sdk/python/overview.md) - 深入 SDK 文档
