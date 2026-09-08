# Changelog

## [0.1.1](https://github.com/JayceChant/goliday/compare/v0.1.0...v0.1.1) (2026-09-08)

> 摘要：内部性能与内存优化——日期判定索引合并为整数键稀疏终态表，统计前缀和改定长内联数组并以 uint8 存储；判定与统计行为完全不变，无需迁移。

### Performance Improvements

* 前缀和改 uint8 存储并收敛为闭区间下标 ([3f8c139](https://github.com/JayceChant/goliday/commit/3f8c139758dbc0b1a94258004d63a61636221f24))
* 查询索引与前缀和改整数键稀疏表与定长内联数组 ([88029da](https://github.com/JayceChant/goliday/commit/88029dad0e7c44521db75b31252708724589795e))
