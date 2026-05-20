## Plan: Web2rss 与 rsseverything 对齐测试（使用 gofeed 解析 RSS）

使用 github.com/mmcdole/gofeed 解析 RSS，提高解析复用；在确定性与 e2e 测试中对齐 rsseverything 输出的前 10 条 item（title/link/description），并提供可反复运行的验证步骤来迭代优化抽取逻辑。

**Steps**
1. 引入 gofeed 作为 RSS 解析器，用于解析 rsseverything 的 RSS 以及 BuildFeedForURL 产出的 RSS（对比项时统一解析口径）。
2. 从 https://rsseverything.com/feed/1538.xml 提取前 10 条 item 的 title/link/description，形成固定期望列表（用于确定性测试）。
3. 新增确定性测试：使用 httptest 提供 LanceDB HTML 夹具（或最小 HTML 片段），用 pattern + 模板生成 RSS，再用 gofeed 解析并断言前 10 条 item 与固定期望列表一致。
4. 新增带开关的 e2e 测试（WEB2RSS_E2E=1）：实时拉取 rsseverything RSS 并用 gofeed 解析期望列表，再与 BuildFeedForURL 的解析结果逐项比对。
5. 如有必要，增加轻量规范化步骤（去空白、解码 HTML 实体、合并连续空格），让抽取结果与 rsseverything 的 description 形式一致。
6. 若你希望 sources.yaml 与测试 pattern 对齐，则同步更新 LanceDB 配置。*optional; depends on your decision*

**Relevant files**
- [internal/web2rss/service_test.go](internal/web2rss/service_test.go) — 添加确定性对齐测试
- [internal/web2rss/e2e_test.go](internal/web2rss/e2e_test.go) — 添加 rsseverything 对照 e2e 测试
- [internal/web2rss/sources.yaml](internal/web2rss/sources.yaml) — 可选更新 LanceDB pattern
- go.mod / go.sum — 新增 gofeed 依赖

**Verification**
1. 运行 go test ./internal/web2rss 验证确定性测试。
2. 运行 WEB2RSS_E2E=1 go test ./internal/web2rss -run RSSEverything 验证对照 e2e。
3. 反复运行对照测试（例如循环 5-10 次）观察真实值与期望值差异，以便迭代优化抽取逻辑和规范化规则。

**Decisions**
- 使用 gofeed 统一解析 RSS 输出与对照源。
- 对比范围：前 10 条 item 的 title/link/description。
- rsseverything 作为 e2e 对照源，采用带开关的实时拉取。

**Further Considerations**
1. rsseverything 输出可能随时间变化，需在 e2e 失败时更新固定期望列表或调整规范化规则。