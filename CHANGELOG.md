# Changelog

## [0.2.1](https://github.com/JayceChant/goliday/compare/v0.2.0...v0.2.1) (2026-09-25)


### Bug Fixes

* proto 工具链改用 go.mod tool 指令经 go.sum 锁定安装 ([f3282d8](https://github.com/JayceChant/goliday/commit/f3282d8443ea69df984ad6ffa9bb90c459f94bfa))

## [0.2.0](https://github.com/JayceChant/goliday/compare/v0.1.1...v0.2.0) (2026-09-24)


### Features

* healthz 增加版本与配置加载时间字段 ([f39c1e0](https://github.com/JayceChant/goliday/commit/f39c1e0221865322ab028640c375469562fc1971))


### Bug Fixes

* 500 兜底错误记录底层原因便于排查 ([e3e1d1b](https://github.com/JayceChant/goliday/commit/e3e1d1b995a8fc7d2918074c9f11f49f8b36d85a))
* HTTP 服务端补充请求头与空闲连接超时 ([b4dae6d](https://github.com/JayceChant/goliday/commit/b4dae6daee489429853f9f6687cc6a55e3682ef9))
* Stats 改内部排序去重并修复年份哨兵漏判 ([7c8d997](https://github.com/JayceChant/goliday/commit/7c8d9973bd2b517d3c9c5522f1bdbbdc12e78eb0))
* 前缀和计数改 uint16 修复无调整年溢出回绕 ([406424f](https://github.com/JayceChant/goliday/commit/406424f36aa17b4a03fcc9afacd2a08bb418e0bd))
* 拆分 go install 逐模块安装 buf 与 protoc 插件 ([3e722fc](https://github.com/JayceChant/goliday/commit/3e722fcbc5b61db4de2e7ac6eaad17bfb95a0e69))
* 服务版本号改为构建期注入消除发版漂移 ([343545c](https://github.com/JayceChant/goliday/commit/343545c47b85dce405d26d1173bfbd27ac42f9d8))

## [0.1.1](https://github.com/JayceChant/goliday/compare/v0.1.0...v0.1.1) (2026-09-08)

> 摘要：内部性能与内存优化——日期判定索引合并为整数键稀疏终态表，统计前缀和改定长内联数组并以 uint8 存储；判定与统计行为完全不变，无需迁移。

### Performance Improvements

* 前缀和改 uint8 存储并收敛为闭区间下标 ([3f8c139](https://github.com/JayceChant/goliday/commit/3f8c139758dbc0b1a94258004d63a61636221f24))
* 查询索引与前缀和改整数键稀疏表与定长内联数组 ([88029da](https://github.com/JayceChant/goliday/commit/88029dad0e7c44521db75b31252708724589795e))
