## Plan: web2rss 部署前清理

TL;DR - 删除示例数据文件，移动模块接口到 internal（`internal/module.go`），并从 `config.go` 移除 `Web2RssSourcesPath` 环境变量；运行静态检查并清理 web2rss 内部无用代码。

**Steps**
1. 删除示例 sources 文件：使用 `git rm` 删除 [internal/web2rss/sources.sample.yaml](internal/web2rss/sources.sample.yaml)。确保 [internal/web2rss/sources.yaml](internal/web2rss/sources.yaml) 留在仓库并有效。
2. 模块包决策（迁移）：将模块接口文件移动到 `internal/module.go`（保留接口设计并在实现时决定包名），并更新所有引用以指向新的位置。理由：保留共享模块接口的好处，同时简化路径层级以便在 `internal` 根目录管理模块注册。
3. 配置变更：从 `internal/config/config.go` 中移除 `Web2RssSourcesPath` 字段及读取 `WEB2RSS_SOURCES_PATH` 的逻辑。改动后默认使用项目内置的 `internal/web2rss/sources.yaml`，如需覆盖可通过文档说明替代方式。文件参考：[internal/config/config.go](internal/config/config.go)
4. 清理 web2rss 无用代码：
   - 运行 `go vet`, `go test ./...`, `go build ./...` 捕获编译/测试/运行时错误。
   - 运行 `golangci-lint run`（或 `staticcheck`/`unused`）查找未使用的函数、变量、导入、死代码。
   - 按 linter 结果：删除/重命名无用函数或变量，移除多余的日志/注释；优先修改这些文件： [internal/web2rss/service.go](internal/web2rss/service.go), [internal/web2rss/handler.go](internal/web2rss/handler.go), [internal/web2rss/module.go](internal/web2rss/module.go)
   - 重新运行测试和构建以验证无回归。
5. 提交流程：
   - 创建专门的 cleanup 分支，例如 `cleanup/web2rss`。
   - 按步骤提交：删除 sample 文件、代码清理、修复 lint 问题、运行测试并在通过后合并到主分支。

**Relevant files**
- [internal/web2rss/sources.sample.yaml](internal/web2rss/sources.sample.yaml)
- [internal/web2rss/sources.yaml](internal/web2rss/sources.yaml)
- [internal/web2rss/service.go](internal/web2rss/service.go)
- [internal/web2rss/handler.go](internal/web2rss/handler.go)
- [internal/web2rss/module.go](internal/web2rss/module.go)
- [internal/module.go](internal/module.go)
- [internal/config/config.go](internal/config/config.go)

**Verification**
1. `git status` 显示 sample 文件已删除并已暂存提交。
2. `go test ./...` 全部通过。
3. `go build ./...` 成功构建二进制（本地或 CI）。
4. `golangci-lint run` 报告为零或仅剩可忽略规则项。
5. 在本地运行一次 web2rss 示例请求，确保 `sources.yaml` 可被读取并生成 RSS。

**Decisions / Assumptions**
- 将 `internal/modules` 移动到 `internal/module.go`（保留接口设计，但路径更扁平）。
- 从 `config.go` 中移除 `Web2RssSourcesPath`，不再使用 `WEB2RSS_SOURCES_PATH` 环境变量。
- 删除 sample 文件不会影响运行，因为 `sources.yaml` 已存在。

**Further Considerations**
1. 是否需要我现在在仓库上执行这些变更（删除 sample、运行 linter 和 tests）并提交到新分支？
2. 是否希望我同时把 `sources.sample.yaml` 的内容记录到 README 作为示例？
