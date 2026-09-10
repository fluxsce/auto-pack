# 1.0 → 2.0 迁移

2.0 **不能**直接打开 1.0 的 `autopack.db`。两边默认都是 `./data/db/autopack.db`，但表名和模型不同。2.0 请用**新库文件**。

可自动搬的是**工作空间 + 工程定义**。环境凭据、成员、作业历史、审计要另做。

## 1. 准备 2.0

1. 复制 `server/configs/app.yaml.example` 为 `server/configs/app.yaml`
2. `database.path` 换成新文件，例如 `./data/db/autopack-v2.db`（不要指向 1.0 库）
3. 改生产 `crypt.passphrase`、`web.jwt_secret`
4. 启动 2.0，确认能登录（默认 `admin` / `123456`）

## 2. 导出 / 导入工程（控制台）

1.0 仍对着旧库跑：每个工作空间右键 **导出**，得到 `format_version: 1` 的 JSON。

2.0：**导入空间**，选该 JSON（`new` 建新空间，或 `replace` 覆盖已有空间工程）。

2.0 会把 1.0 的 `config_json` 转成 `recipe_json`，并保留工程里引用的环境 `rec_id`。导入**不会**带上环境密码。

## 3. 导出 / 导入环境（本脚本）

1.0 环境在全局表 `AP_SYS_ENV`，密码是明文 JSON。2.0 环境仍是全局（`AP_ENV`），整段加密，不跟工作空间绑定。

整库一条命令（工作空间 + 工程 + 5 条全局环境）：

```powershell
# 2.0 必须已启动，且 database.path 不是这份 1.0 库
py -3 -X utf8 script/migrate/v1_envs.py push-db --db "C:\Users\17512\Downloads\autopack.db" --base http://127.0.0.1:8080 --user admin --password 123456
```

先从 1.0 库抽出环境（含密码，文件请当密钥保管）：

```powershell
py -3 -X utf8 script/migrate/v1_envs.py export --db "D:\old-autopack\data\db\autopack.db" --out v1-envs.json
```

再导入全局环境（`--workspace` 只用于登录后的权限校验，数据不挂到该空间）：

```powershell
py -3 -X utf8 script/migrate/v1_envs.py import --file v1-envs.json --base http://127.0.0.1:8080 --workspace "<任意空间rec_id>" --user admin --password "<密码>"
```

脚本按 **1.0 的 rec_id** 写入，这样工程里的环境绑定不用改。`nodejs` 会写成 2.0 的 `node`。

1.0 没有单独的 JDK 环境。本机工具链是 Maven / Node，和 Git / FTP / SMTP 一起在设置「SDK 环境」里，全平台一份。

## 4. 源码检出目录

1.0：`data/workspaces/{空间编码}/...` 或空间上的 `root_dir`  
2.0：`workspace.root/{空间编码}/{工程相对路径}/`（默认 `./data/workspaces`）

把旧检出拷过去后，工程目录页才能直接看到文件。

## 5. 不会自动迁移

- 作业 / 构建历史、发版批次、POM / 克隆分支记录
- 操作审计
- 成员与角色（2.0 新表；导入人成为空间所有者）
- 1.0 的 `project_version`（同名多版本会收成一个工程名）
