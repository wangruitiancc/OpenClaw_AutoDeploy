# OpenClaw AutoDeploy 用户手册

欢迎使用 OpenClaw AutoDeploy。本手册介绍作为租户使用该系统所需了解的一切。无需技术背景——只需按照以下步骤操作即可。

---

## 目录

1. [什么是 OpenClaw AutoDeploy？](#1-什么是-openclaw-autodeploy)
2. [准备工作](#2-准备工作)
3. [安装 CLI 工具](#3-安装-cli-工具)
4. [连接系统](#4-连接系统)
5. [首次登录](#5-首次登录)
6. [创建租户账号](#6-创建租户账号)
7. [配置 API 密钥](#7-配置-api-密钥)
8. [配置你的 Profile](#8-配置你的-profile)
9. [部署容器](#9-部署容器)
10. [查看部署状态](#10-查看部署状态)
11. [访问你的服务](#11-访问你的服务)
12. [停止和启动服务](#12-停止和启动服务)
13. [查看部署历史](#13-查看部署历史)
14. [更新配置](#14-更新配置)
15. [常见问题与解决方案](#15-常见问题与解决方案)
16. [常见问题 FAQ](#16-常见问题-faq)

---

## 1. 什么是 OpenClaw AutoDeploy？

OpenClaw AutoDeploy 是一个平台，可以自动在云端为你设置和管理个人 AI 助手容器。可以把它想象成租用一台预配置好 AI 助手环境的电脑——你无需担心服务器、网络或技术设置。

**你将获得：**
- 一个可访问的 AI 助手容器，有独特的网址
- API 密钥的安全存储（你不需要自己管理）
- 自动健康监控——如果出现问题，系统会尝试修复
- 无停机更新 AI 助手的能力

---

## 2. 准备工作

你需要准备：

- **一台电脑**（Windows、Mac 或 Linux）
- **网络连接**
- **AI 服务商的 API 密钥**（如 OpenAI、Anthropic、MiniMax）——这是你的 AI 助手用来思考的
- **OpenClaw AutoDeploy 的访问凭证**（向管理员索取你的用户名/token）

---

## 3. 安装 CLI 工具

CLI 工具是一个名为 `openclawctl` 的命令行程序，让你控制一切。按照以下步骤安装。

### 3.1 下载工具

向管理员索取下载链接，或从 GitHub 仓库的 releases 页面下载。

### 3.2 在 Mac 或 Linux 上安装

打开终端应用，运行：

```bash
# 给文件执行权限
chmod +x openclawctl

# 移动到一个系统可以找到的目录
sudo mv openclawctl /usr/local/bin/
```

### 3.3 在 Windows 上安装

1. 下载 `.exe` 文件
2. 放在一个你容易找到的文件夹里（如 `C:\Users\你的用户名\OpenClaw\`）
3. 打开命令提示符，导航到那个文件夹：
   ```cmd
   cd C:\Users\你的用户名\OpenClaw
   ```

### 3.4 验证安装

安装后，运行以下命令确认工作正常：

```bash
openclawctl version
```

你应该能看到版本信息。如果显示"命令未找到"，尝试重启终端应用。

---

## 4. 连接系统

在执行任何操作之前，你需要告诉 `openclawctl` 去哪里找 OpenClaw 服务器。

### 4.1 获取服务器 URL

向管理员索取服务器 URL。类似这样：
```
https://api.openclaw.example.com
```

### 4.2 配置 CLI

运行以下命令保存你的设置（把 URL 换成你实际的服务器地址）：

```bash
openclawctl config init
openclawctl config set server https://api.openclaw.example.com
```

### 4.3 保存你的令牌

管理员给了你一个 bearer token。按以下方式安全保存：

```bash
openclawctl config set token-file ~/.config/openclawctl/token
```

然后创建令牌文件：

```bash
# 在 Mac/Linux 上：
echo "你的令牌内容" > ~/.config/openclawctl/token
chmod 600 ~/.config/openclawctl/token
```

**重要：** 令牌不要分享给他人。它是访问你账户的钥匙。

---

## 5. 首次登录

运行以下命令，确认系统可连通：

```bash
openclawctl health
```

你应该看到系统健康的输出。

也检查一下凭证是否有效：

```bash
openclawctl ready
```

如果两个命令都成功，说明已连接并通过认证。

---

## 6. 创建租户账号

"租户"（Tenant）是你在 OpenClaw 系统中的账号。在部署任何东西之前，你需要先创建一个。

### 6.1 获取你的用户 ID

向管理员索取你的 external user ID，通常类似 `user_10001`。

### 6.2 创建租户

运行以下命令（把值替换成你自己的）：

```bash
openclawctl tenant create \
  --external-user-id user_10001 \
  --slug my-name-001 \
  --display-name "我的 OpenClaw 账号"
```

参数说明：
- `--external-user-id`：管理员给你的用户 ID
- `--slug`：你账号的简短唯一昵称（不能有空格，用连字符）
- `--display-name`：在界面中显示的友好名称

### 6.3 验证创建成功

列出所有租户确认：

```bash
openclawctl tenant list
```

你应该能看到新创建的租户。

---

## 7. 配置 API 密钥

你的 AI 助手需要一个 API 密钥才能工作。你把它安全地存储在系统中——输入后它不会再以明文形式出现。

### 7.1 获取 API 密钥

在 AI 服务商注册并获取 API 密钥，例如：
- **OpenAI**：https://platform.openai.com/api-keys
- **Anthropic**：https://console.anthropic.com/
- **MiniMax**：https://platform.minimaxi.com/

复制密钥——它看起来像一串字母和数字的组合（`sk-xxxx...`）。

### 7.2 安全存储

告诉系统保存你的 API 密钥（系统会从环境变量中读取）：

```bash
export OPENAI_API_KEY="sk-xxxx_你的实际密钥"
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-env OPENAI_API_KEY
```

或者从文件保存：

```bash
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-file ./my-api-key.txt
```

**安全提示：** 你的实际 API 密钥不会出现在命令输出或日志中。

### 7.3 验证存储成功

列出你的密钥确认（不会看到实际密钥，只看到名称）：

```bash
openclawctl secret list --tenant my-name-001
```

---

## 8. 配置你的 Profile

你的 Profile 告诉系统如何设置你的 AI 助手容器。

### 8.1 设置基本 Profile

运行以下命令（向管理员询问正确的值）：

```bash
openclawctl profile set --tenant my-name-001 \
  --template tpl_standard \
  --tier standard \
  --route-key my-name-001 \
  --model-provider openai-compatible \
  --model-name gpt-4.1
```

参数说明：
- `--template`：使用哪个模板（询问管理员）
- `--tier`：资源等级（standard、large 等）
- `--route-key`：唯一密钥，会成为你服务 URL 的一部分
- `--model-provider`：AI 服务商类型
- `--model-name`：使用哪个 AI 模型

### 8.2 验证 Profile

在部署前，确保一切配置正确：

```bash
openclawctl profile validate --tenant my-name-001
```

如果验证通过，就可以部署了。

---

## 9. 部署容器

现在你已准备好启动你的 AI 助手。部署会创建你的容器并开始运行。

```bash
openclawctl deployment deploy --tenant my-name-001
```

系统将：
1. 在云端创建你的容器
2. 启动它
3. 验证它是否正常工作

这通常需要 1-3 分钟。

### 等待部署完成

如果你想等待并查看结果：

```bash
openclawctl deployment deploy --tenant my-name-001 --wait --wait-timeout 180s
```

命令会持续检查直到部署成功或失败。

---

## 10. 查看部署状态

### 10.1 查看当前实例

查看你租户当前运行的是什么：

```bash
openclawctl instance get --tenant my-name-001
```

### 10.2 查看待处理任务

如果部署仍在进行中：

```bash
openclawctl job list --tenant my-name-001 --status pending
```

### 10.3 监视特定任务

监视某个任务的进度：

```bash
openclawctl job watch --job ee18a31b-28a4-4937-9f42-6b78a0fda48f
```

（把 job ID 换成你要监视的那个）

---

## 11. 访问你的服务

部署完成后，你的 AI 助手可以通过一个独特的网址访问：

```
http://my-name-001.localtest.me
```

（把 `my-name-001` 换成你实际的 route key，把 `localtest.me` 换成你实际的域名）

在浏览器中打开这个地址来使用你的 AI 助手。

### 如果无法访问

1. 等 2-3 分钟——启动需要时间
2. 检查部署是否成功（`openclawctl instance get --tenant my-name-001`）
3. 查看 [常见问题与解决方案](#15-常见问题与解决方案)

---

## 12. 停止和启动服务

### 停止服务

不需要时停止你的容器（节省资源）：

```bash
openclawctl deployment stop --tenant my-name-001
```

你的数据会被保留。

### 重新启动服务

```bash
openclawctl deployment start --tenant my-name-001
```

### 重启服务

如果感觉有问题：

```bash
openclawctl deployment restart --tenant my-name-001
```

---

## 13. 查看部署历史

查看所有过去的部署：

```bash
openclawctl instance history --tenant my-name-001
```

这显示了你每次部署、停止或更新的记录——排查问题时很有用。

---

## 14. 更新配置

### 更换 API 密钥

如果你的 AI 服务商密钥过期或更换了：

```bash
# 设置新密钥
export NEW_API_KEY="sk-xxxx_新密钥"
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-env NEW_API_KEY
```

### 用新设置重新部署

更改 Profile 后：

```bash
openclawctl deployment redeploy --tenant my-name-001 --strategy replace
```

这会用更新后的配置替换当前容器。

### 更新 Profile

更改 AI 模型或其他设置：

```bash
openclawctl profile set --tenant my-name-001 \
  --template tpl_standard \
  --tier standard \
  --route-key my-name-001 \
  --model-provider openai-compatible \
  --model-name gpt-4.1
```

然后重新部署。

---

## 15. 常见问题与解决方案

### 问题：显示"命令未找到：openclawctl"

**原因：** 工具未安装或不在系统路径中。

**解决方案：**
1. 找到你保存文件的目录
2. 使用完整路径，如 `/usr/local/bin/openclawctl`
3. 或将该目录添加到系统 PATH

### 问题："认证失败"或"令牌无效"

**原因：** 令牌错误或过期。

**解决方案：**
1. 联系管理员获取新令牌
2. 更新令牌文件：
   ```bash
   echo "新令牌" > ~/.config/openclawctl/token
   ```

### 问题："租户未找到"

**原因：** 租户不存在或 slug 错误。

**解决方案：**
1. 列出所有租户找到正确的 slug：
   ```bash
   openclawctl tenant list
   ```

### 问题："部署失败"或容器无法启动

**原因：** 配置错误或资源问题。

**解决方案：**
1. 检查任务状态：
   ```bash
   openclawctl job list --tenant my-name-001 --status pending
   ```
2. 验证 Profile：
   ```bash
   openclawctl profile validate --tenant my-name-001
   ```
3. 检查 API 密钥是否有效
4. 联系管理员

### 问题："密钥未找到"

**原因：** 还没有设置 API 密钥。

**解决方案：**
```bash
openclawctl secret set --tenant my-name-001 OPENAI_API_KEY --from-env OPENAI_API_KEY
```

### 问题：无法通过网址访问服务

**原因：** 服务未就绪或网址错误。

**解决方案：**
1. 检查容器是否在运行：
   ```bash
   openclawctl instance get --tenant my-name-001
   ```
2. 等待 2-3 分钟启动
3. 向管理员确认网址
4. 检查健康状态：
   ```bash
   curl http://127.0.0.1:8080/healthz
   ```

### 问题：验证失败

**原因：** 缺少必需的配置。

**解决方案：**
1. 查看缺少什么——通常是模板、API 密钥或 route key
2. 确保所有必填字段都已设置
3. 向管理员询问正确的值

---

## 16. 常见问题 FAQ

### 问：我的 API 密钥安全吗？
**答：** 安全。你的 API 密钥是加密存储的。它不会出现在日志或命令输出中。

### 问：如果我的 AI 服务商宕机了怎么办？
**答：** 在服务商恢复之前，你的 AI 助手无法工作。系统会在恢复后自动重试。

### 问：我可以使用多个 AI 服务商吗？
**答：** 可以。询问管理员如何配置多个服务商。

### 问：如果我需要更多资源怎么办？
**答：** 联系管理员升级你的资源等级。

### 问：我可以销毁部署重新开始吗？
**答：** 可以：
```bash
openclawctl deployment destroy --tenant my-name-001 --yes
```
这会删除你的容器。你的 Profile 和密钥会保留。

### 问：如何知道我的容器是否健康？
**答：** 检查实例：
```bash
openclawctl instance get --tenant my-name-001
```

### 问：停止和销毁有什么区别？
**答：** **停止**会暂停你的容器并保留数据。**销毁**会完全删除容器。

### 问：遇到问题联系谁？
**答：** 联系你的系统管理员。如果他们无法解决，会转交给 OpenClaw 团队处理。
