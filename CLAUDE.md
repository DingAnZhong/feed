# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

## 5. 项目特定指南

### 5.1 测试要求

**所有代码变更必须包含测试覆盖：**
- Service 层：每个 public 函数至少一个正向测试 + 一个负向测试
- Repository 层：每个 CRUD 操作至少一个测试
- 使用 Snowflake ID 避免测试数据冲突
- 测试数据隔离：使用 ID >= 3000 的数据范围

**测试运行：**
```bash
# 运行所有测试
go test ./...

# 运行特定包
go test -v ./internal/service/...
go test -v ./internal/repository/...
```

### 5.2 代码规范

**必须遵守的规则：**
- 所有数据库操作使用 Context 传递
- Redis 操作失败时降级到数据库（fail-over）
- 限流器 Redis 不可用时放行请求（fail-open）
- 使用 `snowflake.GenerateID()` 生成主键 ID
- 关注/取消关注使用 `FollowUser` 函数统一处理

**数据库操作：**
- 批量查询使用 `GetPostsByIDs`（支持 `FIELD()` 排序）
- 粉丝列表查询使用 `GetFollowerIDs`（分批 500）
- 计数操作优先从 Redis 获取，失败则降级 DB

### 5.3 技术栈

后端技术栈：go+gin+gorm+mysql+redis+kafka

| 组件 | 版本 | 用途 |
|------|------|------|
| Go | 1.25 | 主语言 |
| Gin | 1.12 | HTTP 框架 |
| GORM | 1.31 | ORM |
| MySQL | 8.0 | 持久化存储 |
| Redis | 7.0 | 缓存 + ZSet |
| Kafka | 3.7 | 异步消息队列 |

### 5.4 压测基准

**性能目标：**
- 发帖接口：< 50ms P99
- Feed 拉取：< 20ms P99
- 关注操作：< 10ms P99

**压测运行：**
```bash
# 运行压测
go test -bench=. -benchtime=10s ./test/...
```

---

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
