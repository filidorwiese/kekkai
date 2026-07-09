package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kekkai/internal/docker"
)

// Traffic streams a live, labeled log of the sandbox's egress for $PWD
// (specs/010, renamed by specs/013). Two in-container tcpdump readers attach
// to the observe-only NFLOG groups the firewall installs (§9); group
// membership IS the verdict (1 = allowed + DNS, 2 = blocked), so no fragile
// prefix parsing. Traffic never modifies firewall rules or container state
// (FR-005).
func Traffic() (int, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}
	containers, err := docker.ContainersByLabel(LabelCwd + "=" + pwd)
	if err != nil {
		return 1, err
	}
	containerID := ""
	for _, c := range containers {
		if c.Running {
			containerID = c.ID
			break
		}
	}
	if containerID == "" {
		return 1, fmt.Errorf("no running sandbox for %s, run 'kekkai up'", pwd)
	}

	fmt.Fprintf(os.Stderr, "watching egress of sandbox for %s (Ctrl+C to stop)\n", pwd)

	type readerExit struct {
		err    error
		stderr *bytes.Buffer
	}
	events := make(chan event, 64)
	exits := make(chan readerExit, 2)

	var cmds []*exec.Cmd
	for _, r := range []struct {
		group   int
		verdict string
	}{{1, "ALLOW"}, {2, "BLOCK"}} {
		// -l line-buffered, -n no reverse DNS, -tt epoch timestamps.
		cmd := exec.Command("docker", "exec", "-u", "root", containerID,
			"tcpdump", "-l", "-n", "-tt", "-i", fmt.Sprintf("nflog:%d", r.group))
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return 1, err
		}
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		if err := cmd.Start(); err != nil {
			return 1, err
		}
		cmds = append(cmds, cmd)
		go func(verdict string, cmd *exec.Cmd, out io.Reader, stderr *bytes.Buffer) {
			sc := bufio.NewScanner(out)
			for sc.Scan() {
				events <- event{verdict: verdict, line: sc.Text()}
			}
			// Scan drained before Wait so no buffered output is lost.
			exits <- readerExit{err: cmd.Wait(), stderr: stderr}
		}(r.verdict, cmd, stdout, stderr)
	}

	killReaders := func() {
		for _, c := range cmds {
			_ = c.Process.Kill()
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	w := newWatcher()
	for {
		select {
		case ev := <-events:
			if line, ok := w.render(ev); ok {
				fmt.Println(line)
			}
		case <-sig:
			// Interrupt is the normal way to end a stream. The docker CLI
			// does not forward signals to exec'd processes (feature 009), so
			// the root tcpdumps must be pkilled explicitly or they linger in
			// the sandbox (FR-006). Best-effort: sandbox may be gone already.
			killReaders()
			_ = exec.Command("docker", "exec", "-u", "root", containerID,
				"pkill", "-x", "tcpdump").Run()
			return 0, nil
		case re := <-exits:
			killReaders()
			// 126/127 from docker exec = tcpdump missing in the image.
			if code := exitCode(re.err); code == 126 || code == 127 {
				fmt.Fprintln(os.Stderr, "sandbox image predates 'kekkai traffic'; run 'kekkai down' and 'kekkai up' to rebuild")
				return 1, nil
			}
			// tcpdump announces `listening on nflog:N` once capture is
			// attached; a reader dying without it never captured — relay its
			// own reason (e.g. kernel lacks nflog on some macOS runtimes).
			msg := strings.TrimSpace(re.stderr.String())
			if msg != "" && !strings.Contains(msg, "listening on") {
				fmt.Fprintln(os.Stderr, msg)
				return 1, nil
			}
			fmt.Fprintln(os.Stderr, "sandbox stopped")
			return 1, nil
		}
	}
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 0
}

// event is one raw tcpdump line tagged with the NFLOG-group verdict.
type event struct {
	verdict string // ALLOW or BLOCK
	line    string
}

// watcher holds the session's display state (data-model.md): an IP→hostname
// cache fed by DNS answers, and the query names pending an answer, keyed by
// DNS transaction id (tcpdump's answer decode does not repeat the name).
// Nothing is persisted; render runs single-threaded in the event loop.
type watcher struct {
	hosts   map[string]string // ip → hostname, last-writer-wins
	pending map[string]string // dns txid → queried hostname
	// lastPrinted implements the 5s repeat-suppression window per
	// (verdict, proto, ip, port) tuple. The first occurrence of any tuple
	// always prints — no new destination is ever omitted.
	lastPrinted map[string]time.Time
}

func newWatcher() *watcher {
	return &watcher{
		hosts:       map[string]string{},
		pending:     map[string]string{},
		lastPrinted: map[string]time.Time{},
	}
}

const suppressWindow = 5 * time.Second

// render turns one tcpdump line into a contract line:
// `HH:MM:SS ALLOW|BLOCK <proto> <ip>:<port> [(<hostname>)]` or
// `HH:MM:SS DNS   query|answer ...`. Unparseable lines pass through raw —
// no new destination is ever silently omitted.
func (w *watcher) render(ev event) (string, bool) {
	p, ok := parsePacketLine(ev.line)
	if !ok {
		return ev.line, true
	}
	// Port-53 udp is the DNS tap (group 1), not a connection.
	if p.proto == "udp" && (p.dstPort == "53" || p.srcPort == "53") {
		return w.renderDNS(p)
	}
	tuple := ev.verdict + " " + p.proto + " " + p.dstIP + ":" + p.dstPort
	if last, seen := w.lastPrinted[tuple]; seen && time.Since(last) < suppressWindow {
		return "", false
	}
	w.lastPrinted[tuple] = time.Now()
	line := fmt.Sprintf("%s %-5s %s %s:%s", p.ts, ev.verdict, p.proto, p.dstIP, p.dstPort)
	if h, ok := w.hosts[p.dstIP]; ok {
		line += " (" + h + ")"
	}
	return line, true
}

// renderDNS handles the tcpdump DNS decode. Queries:
// `20403+ A? example.com. (35)`. Answers:
// `20403 2/0/0 CNAME x.net., A 1.2.3.4, A 5.6.7.8 (61)`.
// Only A records matter (the firewall is IPv4-only); AAAA and other
// record types are recognized DNS noise and stay silent (FR-003 wants the
// queried hostname, not every packet).
func (w *watcher) renderDNS(p *packet) (string, bool) {
	f := strings.Fields(p.payload)
	if len(f) == 0 {
		return "", false
	}
	txid := strings.TrimRight(f[0], "+*%$-")
	if p.dstPort == "53" { // query
		for i, t := range f {
			if t == "A?" && i+1 < len(f) {
				name := strings.TrimSuffix(f[i+1], ".")
				if txid != "" {
					w.pending[txid] = name
				}
				return fmt.Sprintf("%s %-5s query %s", p.ts, "DNS", name), true
			}
		}
		return "", false
	}
	// answer: correlate via txid, harvest the A records
	name, known := w.pending[txid]
	var ips []string
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "A" {
			if ip := strings.TrimSuffix(f[i+1], ","); strings.Count(ip, ".") == 3 {
				ips = append(ips, ip)
			}
		}
	}
	if !known || len(ips) == 0 {
		return "", false
	}
	delete(w.pending, txid)
	for _, ip := range ips {
		w.hosts[ip] = name
	}
	return fmt.Sprintf("%s %-5s answer %s -> %s", p.ts, "DNS", name, strings.Join(ips, " ")), true
}

// packet is one dissected tcpdump line.
type packet struct {
	ts               string // HH:MM:SS local
	srcIP, srcPort   string
	dstIP, dstPort   string
	proto            string // tcp or udp
	payload          string // decode text after the destination
}

// parsePacketLine dissects a `tcpdump -l -n -tt` line:
//
//	<epoch> IP <srcip>.<port> > <dstip>.<port>: Flags [S], ... (tcp)
//	<epoch> IP <srcip>.<port> > <dstip>.<port>: UDP, length 32 (udp)
//	<epoch> IP <srcip>.<port> > <dstip>.53: 20403+ A? example.com. (35)
func parsePacketLine(line string) (*packet, bool) {
	f := strings.Fields(line)
	if len(f) < 5 || f[1] != "IP" || f[3] != ">" {
		return nil, false
	}
	epoch, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return nil, false
	}
	srcIP, srcPort, ok := splitHostPort(f[2])
	if !ok {
		return nil, false
	}
	dstIP, dstPort, ok := splitHostPort(strings.TrimSuffix(f[4], ":"))
	if !ok {
		return nil, false
	}
	payload := line[strings.Index(line, f[4])+len(f[4]):]
	p := &packet{
		ts:    time.Unix(int64(epoch), 0).Format("15:04:05"),
		srcIP: srcIP, srcPort: srcPort,
		dstIP: dstIP, dstPort: dstPort,
		payload: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(payload), ":")),
	}
	switch {
	case strings.Contains(payload, "Flags ["):
		p.proto = "tcp"
	case dstPort == "53" || srcPort == "53" || strings.Contains(payload, "UDP"):
		p.proto = "udp"
	default:
		return nil, false // ICMP etc: pass through raw
	}
	return p, true
}

// splitHostPort splits tcpdump's dotted `<ipv4>.<port>` notation.
func splitHostPort(s string) (ip, port string, ok bool) {
	i := strings.LastIndex(s, ".")
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	ip, port = s[:i], s[i+1:]
	if strings.Count(ip, ".") != 3 {
		return "", "", false
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", false
	}
	return ip, port, true
}
