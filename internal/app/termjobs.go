package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 这个文件是"终端的 shell 是哪个进程、前台在跑什么、在哪个目录"：mygo 的终端不给进程号，从应用的子进程里认领
// （pollPids / claimPids）；前台的命令按 ps 读（一次拿到进程组、终端的前台进程组、可执行文件名和命令行）：
// 关格 / 关标签前问一格（TermPane.Job，两次 ps -p），后台轮询 agent 时一次 ps -A 读所有格（foregroundJobs，agents.go），
// 名字都按 jobName 算（node 跑的脚本写的 agent 按脚本名认）；shell 现在的目录按 lsof 读（procCwd）。
// termd 里的 shell 忙不忙由 termd 自己用 TIOCGPGRP 看（termd.proc.busy，"停止终端后台进程"的提示用），
// 那里有不在任何窗口里的会话、应用不知道它们的进程号，所以不走这里。

// 认领 shell 的进程号：mygo 的终端不给进程号，只好看应用的子进程里有终端（tty）的那些
// （git 子进程没有终端），按进程号从小到大分给按启动先后排队的格。认领过的进程号整个进程里不再给别的格
// （测试里一个进程有好几个 App）。还没认领到就关掉的格，closeTerm 等它的 shell 退干净，免得被下一个格认走。
// （试过把记号放进 shell 的环境再用 ps -E 对，本机的 ps 看不到别的进程的环境。）
var (
	claimMu sync.Mutex
	claimed = map[int]bool{}
)

// pollPids 在后台找排队的格的 shell 的进程号，最多找 3 秒。
func (a *App) pollPids() {
	if a.pidPolling || a.Update == nil {
		return
	}
	a.pidPolling = true
	var done atomic.Bool
	up := a.Update
	go func() {
		for i := 0; i < 20 && !done.Load(); i++ {
			time.Sleep(150 * time.Millisecond)
			kids := ttyChildren()
			last := i == 19
			up(func() {
				if a.claimPids(kids) || last {
					done.Store(true)
					a.pidPolling = false
				}
			})
		}
	}()
}

// claimPids 把 kids 里没认领过的分给排队的格，返回还有没有格在等。
func (a *App) claimPids(kids []int) bool {
	a.pidWait = slices.DeleteFunc(a.pidWait, func(p *TermPane) bool { return p.Term == nil || p.pid != 0 })
	claimMu.Lock()
	for _, k := range kids {
		if len(a.pidWait) == 0 {
			break
		}
		if !claimed[k] {
			claimed[k] = true
			a.pidWait[0].pid = k
			a.pidWait = a.pidWait[1:]
		}
	}
	claimMu.Unlock()
	a.syncAgents()
	return len(a.pidWait) == 0
}

// ttyChildren 是应用有终端（tty）的子进程，按进程号排好。
func ttyChildren() []int {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,tty=").Output()
	if err != nil {
		return nil
	}
	me := os.Getpid()
	var kids []int
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[2] == "??" || f[2] == "-" {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		ppid, _ := strconv.Atoi(f[1])
		if ppid == me && pid > 0 {
			kids = append(kids, pid)
		}
	}
	slices.Sort(kids)
	return kids
}

// Job 是这一格前台在跑的命令的名字：没在跑为 ""，不知道 shell 的进程号（还没认领到）为 "?"。
func (p *TermPane) Job() string {
	if p.Term == nil || p.Exited {
		return ""
	}
	if p.pid == 0 {
		return "?"
	}
	return foregroundJob(p.pid)
}

// proc 是 ps 的一行。
type proc struct {
	pgid, tpgid int      // 进程组；它终端的前台进程组（没有终端为 0）
	comm        string   // 可执行文件的名字（ucomm：不带目录，最多 16 字节）
	args        []string // 命令行按空白切开（路径带空格会被切断，所以名字先看 comm）
}

// procCols 是 readProcs 要 ps 打的列（带表头：parseProcs 按表头切）。
const procCols = "pid,pgid,tpgid,ucomm,args"

// readProcs 跑一次 ps，sel 选进程（"-A" 或 "-p", "1,2"），按进程号返回；ps 失败（-p 的进程都没了也算）为 nil。
func readProcs(sel ...string) map[int]proc {
	out, err := exec.Command("ps", append(sel, "-o", procCols)...).Output()
	if err != nil {
		return nil
	}
	return parseProcs(string(out))
}

// parseProcs 读 ps -o procCols 带表头的输出。ucomm 可能带空格（"Claude Helper (R"），ps 把不是最后一列的文字列
// 补成同宽，所以按表头里 UCOMM、ARGS 的位置切；对不上的行（名字里有多字节字符时补齐可能按字宽算）退回按空白切。
func parseProcs(out string) map[int]proc {
	lines := strings.Split(out, "\n")
	cs, as := strings.Index(lines[0], "UCOMM"), strings.Index(lines[0], "ARGS")
	if cs < 0 || as <= cs {
		return nil
	}
	rows := map[int]proc{}
	for _, line := range lines[1:] {
		var comm string
		var args []string
		num := strings.Fields(line[:min(cs, len(line))])
		if len(num) == 3 && len(line) > as && line[as-1] == ' ' && line[cs-1] == ' ' {
			comm, args = strings.TrimSpace(line[cs:as]), strings.Fields(line[as:])
		} else if f := strings.Fields(line); len(f) >= 4 {
			num, comm, args = f[:3], f[3], f[4:]
		} else {
			continue
		}
		pid, err := strconv.Atoi(num[0])
		if err != nil {
			continue
		}
		pgid, _ := strconv.Atoi(num[1])
		tpgid, _ := strconv.Atoi(num[2])
		rows[pid] = proc{pgid, tpgid, comm, args}
	}
	return rows
}

// fgJob 是 shell sh 前台在跑的命令：shell 的进程组不是它终端的前台进程组时，前台那一组组长（在 rows 里找）的名字，
// 组长已经没了为 "?"；在提示符那里等着、只有后台任务（&）的为 ""。
func fgJob(rows map[int]proc, sh proc) string {
	if sh.tpgid <= 0 || sh.tpgid == sh.pgid {
		return ""
	}
	lead, ok := rows[sh.tpgid]
	if !ok {
		return "?"
	}
	return jobName(lead.comm, lead.args)
}

// foregroundJob 是一个 shell（进程号 pid）前台在跑的命令（主线程上关格、关标签之前问）。只看要看的两个进程：
// 两次 ps -p 比一次 ps -A 快好几倍。
func foregroundJob(pid int) string {
	sh, ok := readProcs("-p", strconv.Itoa(pid))[pid]
	if !ok {
		return "" // shell 已经没了
	}
	if sh.tpgid <= 0 || sh.tpgid == sh.pgid {
		return ""
	}
	return fgJob(readProcs("-p", strconv.Itoa(sh.tpgid)), sh)
}

// foregroundJobs 用一次 ps -A 读各个 shell（进程号 pids）前台的命令（后台轮询 agent 用）；在提示符那里的不在表里。
func foregroundJobs(pids []int) map[int]string {
	rows := readProcs("-A")
	if rows == nil {
		return nil
	}
	jobs := map[int]string{}
	for _, pid := range pids {
		if sh, ok := rows[pid]; ok {
			if j := fgJob(rows, sh); j != "" {
				jobs[pid] = j
			}
		}
	}
	return jobs
}

// interpreters 是跑脚本的解释器：前台是它们时按脚本的名字算（脚本写的 agent：node / bun / python 跑的）。
var interpreters = map[string]bool{"node": true, "bun": true, "deno": true, "python": true, "python3": true, "ruby": true}

// jobName 是进程的命令名，comm 是可执行文件名（ucomm），args 是按空白切开的命令行；登录 shell 的 - 去掉。
//   - comm 或 argv[0] 是解释器时：改过进程名的（argv[0] 不是解释器，比如把 process.title 设成 claude 的）取 argv[0]，
//     否则取跑的脚本的名字（跳过 - 开头的参数和 run，去掉扩展名），没有脚本就是解释器的名字；
//   - 否则取 comm（路径带空格时 argv[0] 被切断了，comm 不会）；comm 被截到 16 字节、argv[0] 是它加长的样子时取 argv[0]；
//   - comm 为空时按 argv[0] 算，都没有为 "?"。
func jobName(comm string, args []string) string {
	a0 := ""
	if len(args) > 0 {
		a0 = strings.TrimPrefix(filepath.Base(args[0]), "-")
	}
	name := a0
	if comm != "" {
		name = strings.TrimPrefix(filepath.Base(comm), "-")
	}
	if name == "" {
		return "?"
	}
	if interpreters[name] || interpreters[a0] {
		if a0 != "" && !interpreters[a0] {
			return a0
		}
		for _, arg := range args[min(1, len(args)):] {
			if strings.HasPrefix(arg, "-") || arg == "run" {
				continue
			}
			base := filepath.Base(arg)
			return strings.TrimSuffix(base, filepath.Ext(base))
		}
		if a0 != "" {
			return a0
		}
		return name
	}
	if len(a0) > len(name) && strings.HasPrefix(a0, name) {
		return a0
	}
	return name
}

// procCwd 是进程现在的工作目录（lsof），读不到为 ""。
func procCwd(pid int) string {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if d, ok := strings.CutPrefix(line, "n"); ok && filepath.IsAbs(d) {
			return d
		}
	}
	return ""
}
