# 平台库 DDL

三套方言字段对齐，入口均为各目录 `init.sql`。改表请改 `gen_ddl.py` 后执行：

```powershell
py -3 -X utf8 script/db/gen_ddl.py
```

不要只改其中一套 SQL。

```text
script/db/
  gen_ddl.py
  sqlite/init.sql
  mysql/init.sql
  postgres/init.sql
  {engine}/98_patch.sql
  {engine}/99_seed.sql
```

启动时按 `db.driver` 执行对应 `init.sql`，再执行 `98_patch.sql`（列已存在则跳过）。种子用户由应用层写入（bcrypt），SQL 只预置角色动作。

禁止物理外键。密钥只存密文。
