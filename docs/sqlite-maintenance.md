# SQLite 空间维护

`internal/storage/sqlite` 在打开数据库时确保 `auto_vacuum=INCREMENTAL`（值为 `2`）。新库在创建业务表之前启用；已有 `NONE` 数据库在网关启动、业务服务初始化之前执行一次 `VACUUM`，添加增量回收需要的页面反向引用。迁移失败会明确报错并停止启动。已迁移的数据库重启时不会再做全库重建。

存储层每小时检查一次 `freelist_count × page_size`。空页达到 **64 MiB** 时执行 `incremental_vacuum`：每批最多 **4 MiB**，每轮最多 **256 MiB**，整个检查和回收过程限时 **5 秒**。批次之间释放数据库连接，让前台请求继续使用。繁忙、超时或失败时记日志，下个周期继续；空页会保留供正常写入复用。

维护任务只回收空页，不删除事件、审计、配置、聊天或其他记录，不等同于历史保留策略。任务与 Store 同生命周期，关闭时先取消并等待维护任务退出，再关闭数据库连接。

已有库首次迁移会重建文件，需要额外磁盘空间并延长首次启动时间。日常维护不会再运行全库 `VACUUM`。`.cache` 下的离线备份不属于运行中数据库，不会被后台任务修改；创建紧凑备份可使用 SQLite `VACUUM INTO`。

只读检查：

```sql
PRAGMA auto_vacuum;   -- 应为 2
PRAGMA page_size;
PRAGMA page_count;
PRAGMA freelist_count;
```

机制参考：[SQLite auto_vacuum](https://sqlite.org/pragma.html#pragma_auto_vacuum)、[incremental_vacuum](https://sqlite.org/pragma.html#pragma_incremental_vacuum)。
