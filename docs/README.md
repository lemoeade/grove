# 文档

开发约定和常用命令见仓库根目录的 [AGENTS.md](../AGENTS.md)。这里的文档补充具体说明：

- [architecture.md](architecture.md)：架构总览——分层和依赖、各个包的职责、线程和状态、开发约定
  - [architecture/](architecture/)：按子系统分开的细节（App 和命令、状态文件、Git、暂存、文件树、编辑器、Markdown、终端、外观）
- [testing.md](testing.md)：测试分工、验收标准对应的测试，以及内存测量方法
- [release.md](release.md)：版本号规则、构建、DMG 打包、签名、公证、发布流水线、自动更新和 CI
- [shortcuts.md](shortcuts.md)：完整快捷键表，由命令注册表生成（`TestShortcutDoc -update`），请勿手动修改
- [mygo-pitfalls.md](mygo-pitfalls.md)：开发中遇到的问题和处理方法，按布局、列表、键盘、编辑器、终端、Git 等主题分节
- [terms.md](terms.md)：界面术语清单，README、CHANGELOG、注释都照它写

`screenshots/` 存放 README 使用的截图，由 `TestScreenshot` 生成后选取并复制到这里。
