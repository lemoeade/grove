# 打包与发布

本文介绍 Grove 的版本号规则、构建、DMG 打包、发布（GitHub Actions）、自动更新、签名、公证和 CI。日常开发使用
`go tool mygo build -skip-dmg -skip-notarize -sign -` 即可，见 README 的“构建”一节。

## 版本号

版本号统一取自 `mygo.json` 的 `version`，公开版本从 0.1.0 开始。

- `mygo build` 把它写进 Info.plist（`CFBundleShortVersionString` / `CFBundleVersion`），并用
  `-ldflags -X github.com/egoist/mygo.packageVersion=…` 编进二进制；`main` 启动时
  `version.App = mygo.App.Version()`。`go run` 和测试没有注入版本号，显示为 `dev`。
- 关于窗口显示“版本 x.y.z”，下方列出编译时使用的 mygo 和 Go 版本。首次启动或更新后，
  状态栏提示“欢迎使用 Grove x.y.z” / “已更新到 Grove x.y.z”（记录在 state.json 的 `appVersion` 中，同一版本不再提示）。
  崩溃日志和 `workbench.log` 的第一行也会记录版本号。
- `TestVersions`（`internal/version`）检查两项：根目录 `CHANGELOG.md` 中必须有以 `## v<版本> ` 开头的小节；
  编译时使用的 mygo 版本必须与 go.mod 中锁定的版本一致。

1.0 之前，仅修复问题时递增补丁版本号（0.1.0 → 0.1.1）；新增功能或改变行为时递增次版本号（0.1.x → 0.2.0）。

日常改动不修改版本号。涉及行为变化的 PR 在 `CHANGELOG.md` 顶部的 `## 未发布` 中补充记录，`mygo.json` 保持上次发布的版本号。

发布新版本的步骤：

1. 改 `mygo.json` 的 `version`；
2. 把 `CHANGELOG.md` 的 `## 未发布` 改成 `## v<版本> — <日期>`，在上方新增一个空的 `## 未发布`。
   这一节就是更新窗口里“改了什么”的内容（mygo 把它写进更新清单），写给用户看；
3. `GOTOOLCHAIN=auto go vet ./... && GOTOOLCHAIN=auto go test -count=1 ./... && GOTOOLCHAIN=auto go test -race ./...`；
4. 按下文构建，在真实窗口中手动检查截图无法验证的部分（菜单、对话框、毛玻璃、拖放、终端）；
5. 提交、推到 main，在该提交上打标签并推上去：`git tag v<版本> && git push origin v<版本>`；
6. `release.yml` 建好草稿 Release、传完文件后（十来分钟），在 GitHub 的 Releases 页看一眼草稿，点 Publish。
   发布之后装好的 Grove 才看得到新版本，`install.sh` 也才装它。

## 构建

```bash
GOTOOLCHAIN=auto go tool mygo build -skip-dmg -skip-notarize -sign -
```

构建产物为 `build/darwin-arm64/Grove.app`（约 30 MB），使用 ad hoc 签名，可在构建机器上打开。
`mygo.json` 中的应用名为 Grove，bundle id 为 `io.github.lemoeade.grove`，可执行文件为 `Contents/MacOS/Grove`，DMG 文件名为 `grove-<版本>-arm64.dmg`（mygo 使用应用名的 slug）。
源码中的 `cmd/workbench`、`WORKBENCH_*` 沿用了项目旧名，保留原因见 `AGENTS.md`。
复制到其他 Mac 后会被 Gatekeeper 拦截（见下文“没有 Apple 开发者账号时怎么装”）。

终端后台进程（termd）不单独打包。同一个可执行文件带上 `termd` 参数（`Grove termd <套接字> <日志>`）后，`main` 在第一行将其交给后台进程入口，
不打开窗口，也不显示在 Dock 中。因此，签名和公证仍只需处理这一个可执行文件。

应用图标是 `assets/icon.png`（1024×1024，`mygo.json` 的 `icon`，mygo 转成 `AppIcon.icns`）。它是
`internal/appicon` 用代码绘制的：修改绘制代码后，运行 `go test ./internal/appicon -update` 重新生成；关于窗口使用同一套绘制代码。
图标不放在 `resources/` 中，因为该目录会原样复制到 `Contents/Resources`，放入图标会导致重复打包。

`resources/` 里有字体（JetBrains Mono Nerd Font）和 `licenses/`（第三方许可证全文，关于窗口的"查看许可证全文"打开它）。

## DMG

```bash
GOTOOLCHAIN=auto go tool mygo build -skip-notarize -sign -
```

这条命令会额外生成 `build/darwin-arm64/grove-<版本>-arm64.dmg`，包含“拖到应用程序”的窗口布局。
mygo 的处理流程：`hdiutil create` 创建可写镜像 → `hdiutil attach -readwrite` 挂载并写入 `.DS_Store`（图标位置）→
`detach` → 转成只读的 ULMO 格式。系统有 `diskutil image create from` 时用它转，没有才用 `hdiutil convert`
（macOS 27 上 `hdiutil convert` 会失败）。

部分机器会因挂载权限拦截 `attach`，因此 README 的日常构建命令带有 `-skip-dmg`。遇到这种情况，可以跳过挂载，
直接从目录创建只读镜像，但这样生成的镜像没有窗口布局：

```bash
mkdir -p dist/stage && ditto build/darwin-arm64/Grove.app dist/stage/Grove.app && ln -s /Applications dist/stage/Applications
hdiutil create -srcfolder dist/stage -volname Grove -fs HFS+ -format ULMO -ov dist/Grove-<版本>-arm64.dmg
hdiutil verify dist/Grove-<版本>-arm64.dmg
```

新系统会提示 `hdiutil create -volname -format` 已弃用，建议使用 `diskutil image create`；目前该命令仍可用。
这样生成的 DMG 也可以按下文公证：先运行 `xcrun notarytool submit … --wait`，再运行 `xcrun stapler staple`。

## 发布（GitHub Actions）

`.github/workflows/release.yml`，推 `v*` 标签时跑：

1. **核对、建草稿**（ubuntu）：标签必须等于 `v` + `mygo.json` 的 `version`，`CHANGELOG.md` 里必须有 `## v<版本>` 这一节，
   对不上就停（什么都不建）；然后 `gh release create --draft --verify-tag --generate-notes` 建草稿 Release
   （Release 页的说明是 GitHub 按 PR 生成的，可以在草稿里改；更新窗口用的是 CHANGELOG 那一节，两者不相干）。
2. **构建、上传**（`macos-latest`）：`go vet`、`go test`，按 secrets 选签名身份（下一节），然后

   ```bash
   go tool mygo build -skip-notarize -sign "$SIGN_IDENTITY" -upload
   ```

   传到草稿里的文件：`grove-<版本>-arm64.dmg`、`grove-<版本>-darwin-arm64.tar.gz`（更新包）、
   `grove-<旧版本>-to-<版本>-darwin-arm64.delta` 这样的增量包（从之前发布过的版本，bsdiff）、`update-darwin-arm64.json`（更新清单：
   版本、CHANGELOG 那一节、各个包的地址、大小、Ed25519 签名）、`install.sh`、`SHA256SUMS`。

草稿不会自动发布：看过再点 Publish。重跑这个工作流（Re-run）会替换草稿里的文件。

要设的 secrets（在仓库根目录，用自己的终端跑；私钥、证书、密码都别贴到聊天、issue 或日志里）：

```bash
gh secret set MYGO_UPDATER_PRIVATE_KEY < "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key"
# 可选：自签名证书（见下一节）
base64 -i grove-sign.p12 | gh secret set MACOS_CERTIFICATE
gh secret set MACOS_CERTIFICATE_PASSWORD
gh secret set MACOS_SIGN_IDENTITY --body "Grove Self-Signed"
```

## 自动更新

用 mygo 的 updater（`mygo.json` 的 `updates`，插件 `plugins/updater/native`，原生窗口）：

- **密钥**：`go tool mygo keygen` 生成 Ed25519 密钥对，默认写到 `~/Library/Application Support/mygo/update-keys/`。
  公钥（`mygo-update.pub` 的内容）写在 `mygo.json` 的 `updates.publicKey`，编进应用；私钥（`mygo-update.key`）不进仓库，
  存进密码管理器，再设成 secret `MYGO_UPDATER_PRIVATE_KEY`。**私钥丢了，装好的 Grove 就再也收不到更新**（只认这把钥匙签的包），
  只能让用户重新用 install.sh 装一个换了公钥的版本；私钥泄露了，别人就能给 Grove 用户推“更新”。
- **检查**：正式构建每天在后台查一次 `https://github.com/lemoeade/grove/releases/latest/download/update-darwin-arm64.json`
  （只看发布了的 Release，草稿和预发布不算），有新版本就弹窗显示 CHANGELOG 那一节，用户选“安装”“跳过这个版本”“以后提醒”。
  菜单 Grove → “检查更新…”（命令面板里也有）随时查。设置 → 更新：自动检查、自动下载并安装（下次启动生效）、当前版本和上次检查的时间。
  这两个开关存在 mygo 的 `updater.json`（`~/Library/Application Support/Grove/`），不在 state.json 里。
- **安装**：下载（有增量包先用增量包）→ 验 Ed25519 签名 → 换掉 `/Applications/Grove.app`（或用户装的地方）→ 重启。
  更新包是 curl 式的下载，没有隔离属性（quarantine），所以不经过 Gatekeeper：没公证的构建也能一路自动更新。
- **不检查的情况**：`go run`、`mygo dev` 的开发构建（设置页写“这个构建不能自己更新”，菜单项点了窗口里也这么说）；
  设了 `WORKBENCH_STATE` 的实例不在后台查（量内存、冒烟时不弹窗抢前台），手动“检查更新…”照旧能用。
- 代码：`internal/app/updates.go`（界面只经 `app.Updates` 这组钩子，测试里为 nil）、`cmd/workbench/main.go` 的 `setupUpdates`、
  `internal/ui/settings.go` 的“更新”页。

## 没有 Apple 开发者账号时怎么装

没有账号就不能用 Developer ID 签名、不能公证。Grove 现在就是这样发的：

- **主推 install.sh**（README 里那一行）：

  ```bash
  curl -fsSL https://github.com/lemoeade/grove/releases/latest/download/install.sh | sh
  ```

  它读最新发布的更新清单，下载 tar.gz、验 Ed25519 签名，装到 `/Applications`（写不进去就 `~/Applications`）再打开。
  curl 下的文件没有隔离属性，Gatekeeper 不拦，用户不用点任何“仍要打开”。`sh install.sh --uninstall` 卸载（设置和数据留着）。
- **DMG**：浏览器下载的 DMG 带隔离属性，Gatekeeper 会拦（“无法验证开发者”或“已损坏”）。macOS 15 起右键“打开”不再能绕过，
  要么拖进“应用程序”后先双击一次被拦，再到 系统设置 → 隐私与安全性 → 最下面点“仍要打开”；要么在终端里
  `xattr -dr com.apple.quarantine /Applications/Grove.app`。只第一次需要，之后的自动更新都不经过 Gatekeeper。

### 签名身份：ad hoc 还是自签名证书

- **ad hoc**（`-sign -`，没设证书 secrets 时）：能装、能跑、能自动更新。但 ad hoc 签名的“指定要求”是可执行文件的哈希，
  每个版本都不一样：系统眼里每次更新都是新应用，给过的权限（辅助功能、完全磁盘访问、访问“文稿”等文件夹的提示……）要重新给，
  钥匙串里存的东西也可能再问一次。
- **自签名“代码签名”证书**（推荐）：指定要求钉在证书上（`certificate leaf = H"…"`），每个版本一样，权限给一次就行。
  不需要任何账号，证书系统不信任也没关系（codesign 照签，`find-identity` 标 `CSSMERR_TP_NOT_TRUSTED` 是正常的）；
  它不能让 Gatekeeper 放行，首次安装的方式和 ad hoc 一样。

  用 macOS 自带的 `/usr/bin/openssl`（LibreSSL；Homebrew 的 OpenSSL 3 导出 .p12 时要加 `-legacy`，不然 `security import` 认不出）：

  ```bash
  cat > grove-sign.cnf <<'EOF'
  [req]
  distinguished_name=dn
  x509_extensions=ext
  prompt=no
  [dn]
  CN=Grove Self-Signed
  [ext]
  basicConstraints=critical,CA:false
  keyUsage=critical,digitalSignature
  extendedKeyUsage=critical,codeSigning
  EOF
  /usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -keyout grove-sign.key -out grove-sign.crt -days 7300 -config grove-sign.cnf
  /usr/bin/openssl pkcs12 -export -inkey grove-sign.key -in grove-sign.crt -out grove-sign.p12   # 提示时设一个密码
  ```

  按上一节设 `MACOS_CERTIFICATE` / `MACOS_CERTIFICATE_PASSWORD` / `MACOS_SIGN_IDENTITY`，然后把 `grove-sign.p12` 和密码存进密码管理器，
  删掉 `grove-sign.key` 和 .p12 的本地副本。换证书等于换签名身份，用户要重新给一次权限，所以别轻易换。
  从 ad hoc 切到自签名的那一次更新，用户也要重新给一次。

用真身份（自签名或 Developer ID）签名时，mygo 给 .app 和 `Contents/Resources` 里的代码加 hardened runtime 和安全时间戳。
hardened runtime 默认开库验证：只许加载同一 Team ID 签的库，而自签名证书没有 Team ID，purego 加载 `libghostty-vt.dylib` 就会失败
（终端“启动失败”，“mapping process and mapped file (non-platform) have different Team IDs”）。所以 `mygo.json` 的
`macos.entitlements` 指向 `assets/entitlements.plist`，里面是 `com.apple.security.cs.disable-library-validation`。
本机用自签名证书 + hardened runtime + 这个 entitlement 签过一份，终端能起 shell、单实例锁正常；去掉 entitlement 终端就起不来。
ad hoc 签名不加 hardened runtime，这个 entitlement 对它没有影响。

## 签名和公证（以后有了 Apple 开发者账号）

需要付费的 Apple Developer 账号。以下步骤尚未实际验证，是根据 mygo 代码
（`cmd/mygo/dmg.go`、`bundle.go`）和 Apple 的流程整理的。

1. **证书**：在钥匙串中安装 “Developer ID Application: 名字 (TEAMID)” 证书，用 `security find-identity -v -p codesigning` 确认。
2. **公证凭据**：将凭据存入钥匙串（只需一次；密码使用 appleid.apple.com 生成的 App 专用密码）：

   ```bash
   xcrun notarytool store-credentials grove-notary --apple-id 你的AppleID --team-id TEAMID
   ```

3. **mygo.json**：在 `macos` 中添加以下字段（签名身份也可以不写入文件，构建时通过 `-sign "Developer ID Application: …"` 传入）：

   ```json
   "signingIdentity": "Developer ID Application: 名字 (TEAMID)",
   "notarize": { "keychainProfile": "grove-notary" }
   ```

   配置 `notarize` 后，使用 ad hoc 签名（`-sign -`）构建时必须带 `-skip-notarize`，否则 mygo 会报错。
   README 和 CI 中的命令已包含该参数。
4. **构建**：`GOTOOLCHAIN=auto go tool mygo build`。mygo 用 Developer ID 签名时加 hardened runtime 和安全时间戳，
   同时签名 `Contents/Resources` 中的可执行文件和 dylib（mygo 放入的 libghostty-vt），生成 DMG，再运行 `notarytool submit --wait`。
   通过后，用 `stapler staple` 将公证票据附加到 DMG 和 .app。
5. **核对**：

   ```bash
   codesign -dv --verbose=4 build/darwin-arm64/Grove.app
   spctl -a -vv -t install build/darwin-arm64/grove-<版本>-arm64.dmg
   xcrun stapler validate build/darwin-arm64/grove-<版本>-arm64.dmg
   ```

   验证失败时，用 `xcrun notarytool log <提交 ID> --keychain-profile grove-notary` 查看日志（mygo 出错时会输出这条命令）。

`assets/entitlements.plist` 里关掉库验证是为自签名证书加的；用 Developer ID 时库和应用同一个 Team ID，可以去掉
（公证允许这个 entitlement，留着也能过）。CI 里换成 Developer ID：照 mygo 的 `docs/github-actions.md`，导入 Developer ID 的 .p12
和公证凭据，`release.yml` 的构建去掉 `-skip-notarize`。换签名身份后用户要重新给一次权限；更新包的 Ed25519 密钥不用换。

## 打开方式和单实例

`mygo.json` 的 `macos.infoPlist` 加了 `CFBundleDocumentTypes`：文件夹（`public.folder`）和任何文件（`public.item`），
`LSHandlerRank` 为 `Alternate`：访达的“打开方式”中会列出 Grove，也可以将文件拖到 Dock 图标上，但不会将 Grove 设为默认程序。
系统可能需要等待片刻，或在 .app 移入“应用程序”后，才会识别这些类型。

单实例由 `mygo.App.RequestSingleInstanceLock()` 实现，按应用名和 identifier 创建 unix socket。
第二个 `Grove 路径` 进程把参数和工作目录交给已运行的实例后退出；`open -a Grove 路径` 和拖到 Dock 的文件通过
`OnOpenFile` 处理。测量内存或测试时，设置 `WORKBENCH_STATE` 后默认不获取单实例锁，以免把参数交给日常使用的实例。
测试单实例本身时再加 `WORKBENCH_SINGLE=1`，并先用 `pgrep -fl Grove` 确认没有其他实例。

## CI

发布用的 `release.yml` 见上文“发布（GitHub Actions）”。日常的 `.github/workflows/ci.yml`，`macos-latest`：

- 每次推送到 main 或提交 PR 时，运行 gofmt、`go vet ./...`、`go test -count=1 ./...`、`mygo build -skip-dmg -skip-notarize -sign -`，
  将 .app 打包为 zip，作为构建产物保留 14 天；
- `go test -race ./internal/...` 耗时较长，仅在手动触发时勾选 `race`，或每周一运行一次。

env 中设置了 `GOTOOLCHAIN=auto`（go.mod 要求 1.27.3）；mygo 使用 go.mod 锁定的版本，CI 中不要运行 `go get -u`。
UI 测试和终端测试在 GitHub runner 上运行不稳定时，先检查环境差异：runner 没有登录会话，shell 为 `/bin/zsh`。
CI 未设置 `WORKBENCH_TEST_WS`，因此会跳过读取真实工作区的只读测试。

`.github/workflows/claude.yml`：仓库的 owner、成员或协作者在 issue 或 PR 评论里写 `@claude`，就会在 `macos-latest` 上运行 Claude。
用的是 Claude Agent SDK（`.github/claude/agent.mjs`，版本锁在同目录的 `package-lock.json`），不是 `claude-code-action`：
后者按 Claude Code 计费，console 里的赠送额度不能用。Claude 能读写文件、运行任意命令（Bash 不按命令名细分：它能 `go run` 任意代码，细分拦不住什么，只会白耗轮数），
但那一步拿不到 GitHub token，推不了代码。推送和开 PR 由 `finish.sh` 做：issue 上开新分支 `claude/issue-<号>-<时间>` 并开 PR，同仓库的 PR 上推到 PR 的分支，fork 的 PR 不处理。
没跑完（到了上限或出错）时，做了一半的改动也推到分支上留着（PR 上另开 `claude/wip-pr-<号>-<时间>`），但不开 PR，评论里给看改动的链接。
仓库设置里没开 "Allow GitHub Actions to create and approve pull requests" 时开不了 PR，评论里会给一个手动开 PR 的链接。
需要把 Anthropic API 密钥保存为 Secret `ANTHROPIC_API_KEY`（只给 Claude 那一步；GitHub token 只给准备和收尾两步，`persist-credentials: false`）。
这类事件只读取默认分支上的工作流文件和脚本，所以要合并到 main 后才生效。每次最多 100 轮、约 5 美元（`agent.mjs` 的 `maxTurns`、`maxBudgetUsd`）、60 分钟。
仓库公开，运行日志里只打工具名，Claude 的回复贴回 issue / PR 的评论。
Claude 会跑 `go test`，等于执行 PR 里的代码，所以别在不认识的人提的 PR 上 `@claude`。

## 许可

- 第三方组件的许可证全文位于 `resources/licenses/`（清单见 `README.txt`），可以从关于窗口打开。
- `ghostty.txt` 里是来源说明（libghostty-vt 所用的 Ghostty 提交）和那个提交上 LICENSE 的全文（MIT 要求附上原文）。
  mygo 换了 Ghostty 的提交（`plugins/terminal/mygo-plugin.json`）时，对着新提交的 LICENSE 核一遍。
- Grove 使用 MIT 许可证（根目录 `LICENSE`）。如需在 Info.plist 中写入版权信息，在 `mygo.json` 中添加 `copyright`
  字段，对应 `NSHumanReadableCopyright`。
