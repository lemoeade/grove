# Terms

When the UI, README, CHANGELOG or code comments talk about the same thing, they use the same word. **The UI is the
source of truth**: the English terms below are the strings that actually appear in `internal/i18n/en.json`, and the
Chinese terms are the source strings in `internal/i18n/zh-CN.json` (the keys of `en.json`). English docs (README.md,
CHANGELOG, docs/) use the English column; the Chinese README (README.zh-CN.md) and code comments use the Chinese column.
When the UI renames something, update this table and both READMEs together; define new concepts here first.

Terms in quotes are exact UI strings; the rest are the words to use in prose.

## Git

| English UI term | Chinese UI term (zh-CN) | Meaning / usage |
|---|---|---|
| Staged (group); "Stage" / "Unstage" (actions) | 已暂存；“暂存” / “取消暂存” | Changes in the index. Avoid: cached; 已缓存、staged |
| Unstaged ("Unstaged changes") | 未暂存（“未暂存的改动”） | Changes in the working tree that aren't staged. Avoid: modified; 已修改、未缓存 |
| Untracked (untracked files) | 未跟踪（“未跟踪的文件”） | Files Git doesn't track; in the Git panel they share the "Changes" (改动) group with unstaged changes. Avoid: new files; 新文件、untracked |
| "Stage Hunk", "Discard Hunk", "Stage Selected Lines" | 暂存这一块、丢弃这一块、暂存选中的行 | Per-hunk / per-line operations. Avoid: partial staging; hunk、局部暂存 |
| "Discard Changes" | 丢弃改动 | Throws away working-tree changes (`git restore` / `git clean`, asks first). Avoid: revert, reset, undo changes; 放弃改动、还原改动、回退 |
| "Stash", "Pop Stash", "Apply Stash", "Drop Stash" | 贮藏、弹出贮藏、应用贮藏、丢弃贮藏 | `git stash`. Avoid: 储藏; 暂存 (clashes with stage) |
| "Fetch" / "Pull" / "Push"; "Force Push"; "Sync" ("Sync (Pull, then Push)") | 抓取 / 拉取 / 推送；强制推送；同步（拉取再推送） | fetch / pull / push. Avoid: get, upload; 获取、拉、上传 |
| ahead / behind (↑ / ↓) | 领先 / 落后（↑ / ↓） | Commits ahead of / behind the upstream ("%d ahead", "Commits ahead of / behind the upstream"). Avoid: to push; 超前、待推送 |
| "cherry-pick" / "Revert Commit" | 拣选 / 还原提交 | cherry-pick / revert. Avoid: 挑选、回滚 |
| "Abort" / "Continue" / "Skip" | 中止 / 继续 / 跳过 | An in-progress merge, rebase or cherry-pick, in the banner in the Git panel. Avoid: cancel, resume; 取消、恢复 |
| ours / theirs ("Accept Ours", "Accept All Theirs") | 我的 / 他们的（“用我的”“全部用他们的”） | The two sides of a conflict. Avoid: local / remote; ours / theirs（中文里）、本地 / 远程 |
| "Working tree clean" | 工作区干净 | A repo with no changes at all. Avoid: no changes; 无变更、clean |
| "Needs you"; waiting for you, running, "Ready to merge", "Has changes", idle, "Merged" | 需要你、等你、运行中、可合并、有改动、空闲、已合并 | Worktree states (tooltips: "Agent is waiting for you", "Agent is running", "Agent idle"). |
| "Tags" ("New Tag…", "Delete Tag") | 标签（新建标签…、删除标签） | git tags. Chinese 标签 also means an editor tab; tell them apart by context. Avoid: 中文里写 tag |

## Diff

| English UI term | Chinese UI term (zh-CN) | Meaning / usage |
|---|---|---|
| side by side / unified ("Unified Diff") | 并排 / 统一（“统一 diff”） | Two columns vs. one. Avoid: split, inline; 左右、分栏、内联 |
| "Show only the lines around changes" / full file ("Full-File Diff") | 只看改动附近 / 全文（“全文 diff”） | Show only the changed regions vs. the whole file. Avoid: partial, context; 局部、上下文 |

## UI regions and editor

| English UI term | Chinese UI term (zh-CN) | Meaning / usage |
|---|---|---|
| "Sidebar", right panel ("Right Panel (History)"), bottom panel ("Bottom Panel (Terminal / Output)") | 侧栏、右栏（历史）、底栏（终端 / 输出 / 问题） | The three panels; the bottom panel holds Terminal / Output / Problems. Avoid: left bar, panel area; 左边栏、面板区 |
| "File Tree", "Search", Git panel ("Git Sidebar"), "Outline"; "Open Editors" above the file tree | 文件树、搜索、Git 栏、大纲；文件树上面一段“打开的编辑器” | Sidebar pages. In prose, say "Git panel" (the UI also uses "Git view" / "Git Sidebar"). Avoid: Explorer, Source Control; 资源管理器、源代码管理 |
| editor area; "Split Right" / "Split Down" | 编辑器区；左右分屏 / 上下分屏 | The main area. Avoid: 编辑区 (the Chinese README has used both; the UI says 编辑器区) |
| preview tab / pinned tab ("Pin Tab", "Open File (Pinned)") | 预览标签 / 固定标签（“固定打开文件”） | A tab opened with a single click that the next one replaces / a tab kept after double-clicking or editing. Avoid: temporary tab; 临时标签. Note: a bare "Preview" (预览) in the UI means the Markdown / SVG preview |
| "Command Palette…", "Quick Open", "Go to Symbol…" | 命令面板、快速打开、跳到符号 | Command palette and quick open. Avoid: 命令选项板 |
| "Local history" | 本地历史 | The snapshot kept on every save. Avoid: timeline; 时间线 |
| "Go to Definition", "Find References", "Show Definition on Hover" | 跳到定义、查找引用、悬停时显示定义 | Name-based navigation. Avoid: Find All References; 转到定义、查找所有引用 |
