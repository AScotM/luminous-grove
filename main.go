package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Population int     `json:"population"`
	FPS        int     `json:"fps"`
	Seed       int64   `json:"seed"`
	Mode       string  `json:"mode"`
	Theme      string  `json:"theme"`
	Telemetry  bool    `json:"telemetry"`
	Growth     float64 `json:"growth"`
	Trails     float64 `json:"trails"`
	Duration   int     `json:"duration_seconds"`
	Output     string  `json:"output"`
}
type Agent struct {
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	VX      float64 `json:"vx"`
	VY      float64 `json:"vy"`
	Energy  float64 `json:"energy"`
	Age     float64 `json:"age"`
	Phase   float64 `json:"phase"`
	Species int     `json:"species"`
}
type Branch struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Angle  float64 `json:"angle"`
	Length float64 `json:"length"`
	Depth  int     `json:"depth"`
	Energy float64 `json:"energy"`
}
type Metrics struct {
	CPU       float64   `json:"cpu"`
	Memory    float64   `json:"memory"`
	Pressure  float64   `json:"pressure"`
	Processes int       `json:"processes"`
	Load      float64   `json:"load"`
	Timestamp time.Time `json:"timestamp"`
}
type Snapshot struct {
	Config    Config    `json:"config"`
	Tick      int       `json:"tick"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   Metrics   `json:"metrics"`
	Agents    []Agent   `json:"agents"`
	Branches  []Branch  `json:"branches"`
}
type Cell struct {
	R, G, B float64
	Glow    float64
	Glyph   rune
}
type World struct {
	C        Config
	RNG      *rand.Rand
	Agents   []Agent
	Branches []Branch
	Cells    []Cell
	Tick     int
	Metrics  Metrics
	lastCPU  [2]uint64
	nextID   int
	Births   int
	Deaths   int
	Started  time.Time
}

func clamp(x, a, b float64) float64                  { return math.Max(a, math.Min(b, x)) }
func randomRange(r *rand.Rand, a, b float64) float64 { return a + r.Float64()*(b-a) }
func newWorld(c Config) *World {
	if c.Seed == 0 {
		c.Seed = time.Now().UnixNano()
	}
	w := &World{C: c, RNG: rand.New(rand.NewSource(c.Seed)), Cells: make([]Cell, c.Width*c.Height), Started: time.Now()}
	for i := 0; i < c.Population; i++ {
		w.spawn()
	}
	for i := 0; i < int(float64(c.Width)/9); i++ {
		w.Branches = append(w.Branches, Branch{X: randomRange(w.RNG, 0, float64(c.Width)), Y: float64(c.Height - 2), Angle: -math.Pi/2 + randomRange(w.RNG, -.5, .5), Length: randomRange(w.RNG, 2, 5), Depth: 0, Energy: 1})
	}
	return w
}
func (w *World) spawn() {
	a := Agent{ID: w.nextID, X: randomRange(w.RNG, 1, float64(w.C.Width-2)), Y: randomRange(w.RNG, 1, float64(w.C.Height-2)), VX: randomRange(w.RNG, -.3, .3), VY: randomRange(w.RNG, -.2, .2), Energy: randomRange(w.RNG, .5, 1), Phase: randomRange(w.RNG, 0, 2*math.Pi), Species: w.RNG.Intn(3)}
	w.nextID++
	w.Agents = append(w.Agents, a)
	w.Births++
}
func readMetrics() Metrics {
	m := Metrics{Timestamp: time.Now()}
	if b, e := os.ReadFile("/proc/meminfo"); e == nil {
		var total, available float64
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 2 {
				continue
			}
			v, _ := strconv.ParseFloat(f[1], 64)
			switch f[0] {
			case "MemTotal:":
				total = v
			case "MemAvailable:":
				available = v
			}
		}
		if total > 0 {
			m.Memory = 100 * (total - available) / total
		}
	}
	if b, e := os.ReadFile("/proc/pressure/cpu"); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "some ") {
				for _, field := range strings.Fields(line) {
					if strings.HasPrefix(field, "avg10=") {
						m.Pressure, _ = strconv.ParseFloat(strings.TrimPrefix(field, "avg10="), 64)
					}
				}
			}
		}
	}
	if b, e := os.ReadFile("/proc/loadavg"); e == nil {
		f := strings.Fields(string(b))
		if len(f) > 0 {
			m.Load, _ = strconv.ParseFloat(f[0], 64)
		}
	}
	if entries, e := os.ReadDir("/proc"); e == nil {
		for _, v := range entries {
			if _, e := strconv.Atoi(v.Name()); e == nil {
				m.Processes++
			}
		}
	}
	return m
}
func cpuCounters() [2]uint64 {
	var out [2]uint64
	b, e := os.ReadFile("/proc/stat")
	if e != nil {
		return out
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	f := strings.Fields(line)
	for i, s := range f[1:] {
		v, _ := strconv.ParseUint(s, 10, 64)
		out[0] += v
		if i == 3 || i == 4 {
			out[1] += v
		}
	}
	return out
}
func (w *World) sample() {
	m := readMetrics()
	now := cpuCounters()
	total := now[0] - w.lastCPU[0]
	idle := now[1] - w.lastCPU[1]
	if w.lastCPU[0] > 0 && total > 0 {
		m.CPU = 100 * float64(total-idle) / float64(total)
	}
	w.lastCPU = now
	w.Metrics = m
}
func (w *World) update(dt float64) {
	w.Tick++
	if w.Tick%max(1, w.C.FPS) == 1 {
		w.sample()
	}
	decay := math.Pow(clamp(w.C.Trails, .5, .995), dt*30)
	for i := range w.Cells {
		c := &w.Cells[i]
		c.R *= decay
		c.G *= decay
		c.B *= decay
		c.Glow *= decay
		if c.Glow < .012 {
			c.Glyph = ' '
		}
	}
	activity := 0.
	if w.C.Telemetry {
		activity = clamp(w.Metrics.CPU/100, 0, 1)
	}
	for i := range w.Agents {
		a := &w.Agents[i]
		ax, ay := 0., 0.
		count := 0.
		for j := range w.Agents {
			if i == j {
				continue
			}
			b := &w.Agents[j]
			dx := b.X - a.X
			dy := b.Y - a.Y
			d2 := dx*dx + dy*dy
			if d2 < 65 && d2 > .01 {
				d := math.Sqrt(d2)
				if d < 2.8 {
					ax -= dx / d2 * .11
					ay -= dy / d2 * .11
				} else if a.Species == b.Species {
					ax += dx / d * .008
					ay += dy / d * .008
				}
				count++
			}
		}
		if count > 0 {
			ax /= math.Sqrt(float64(count))
			ay /= math.Sqrt(float64(count))
		}
		t := float64(w.Tick) * dt
		a.Phase += dt * (1.2 + activity*3)
		ax += math.Sin(t*.7+a.Phase)*.018 + randomRange(w.RNG, -.018, .018)
		ay += math.Cos(t*.5+a.Phase)*.015 + randomRange(w.RNG, -.018, .018)
		a.VX = clamp((a.VX+ax)*.985, -.5-activity*.35, .5+activity*.35)
		a.VY = clamp((a.VY+ay)*.985, -.35-activity*.2, .35+activity*.2)
		a.X += a.VX * dt * 30
		a.Y += a.VY * dt * 30
		if a.X < 1 || a.X >= float64(w.C.Width-1) {
			a.VX = -a.VX
			a.X = clamp(a.X, 1, float64(w.C.Width-2))
		}
		if a.Y < 1 || a.Y >= float64(w.C.Height-1) {
			a.VY = -a.VY
			a.Y = clamp(a.Y, 1, float64(w.C.Height-2))
		}
		a.Age += dt
		a.Energy -= dt * (.0015 + activity*.001)
		a.Energy += dt * .001 * math.Max(0, math.Sin(a.Phase))
		w.paint(int(a.X), int(a.Y), .4+.6*math.Abs(math.Sin(a.Phase)), a.Species, '•')
	}
	alive := w.Agents[:0]
	for _, a := range w.Agents {
		if a.Energy > 0 && a.Age < 900 {
			alive = append(alive, a)
		} else {
			w.Deaths++
		}
	}
	w.Agents = alive
	target := w.C.Population + int(activity*float64(w.C.Population)*.3)
	if len(w.Agents) < target && w.RNG.Float64() < clamp(w.C.Growth*dt, 0, 1) {
		w.spawn()
	}
	if len(w.Agents) > target*2 {
		w.Agents = w.Agents[:target*2]
	}
	if w.Tick%max(1, w.C.FPS/2) == 0 {
		w.grow()
	}
	w.paintBranches()
}
func (w *World) grow() {
	if len(w.Branches) > w.C.Width*4 {
		return
	}
	n := len(w.Branches)
	for i := 0; i < n; i++ {
		b := w.Branches[i]
		if b.Depth >= 5 || b.Energy < .2 || w.RNG.Float64() > .13*w.C.Growth {
			continue
		}
		x := b.X + math.Cos(b.Angle)*b.Length
		y := b.Y + math.Sin(b.Angle)*b.Length
		if x < 1 || x > float64(w.C.Width-2) || y < 1 {
			continue
		}
		w.Branches = append(w.Branches, Branch{X: x, Y: y, Angle: b.Angle + randomRange(w.RNG, -.7, .7), Length: b.Length * randomRange(w.RNG, .65, .9), Depth: b.Depth + 1, Energy: b.Energy * .82})
	}
}
func (w *World) paint(x, y int, brightness float64, species int, glyph rune) {
	if x < 0 || x >= w.C.Width || y < 0 || y >= w.C.Height {
		return
	}
	c := &w.Cells[y*w.C.Width+x]
	pal := [3][3]float64{{100, 240, 185}, {100, 155, 255}, {255, 190, 100}}
	if w.C.Theme == "moon" {
		pal = [3][3]float64{{135, 200, 255}, {190, 145, 255}, {100, 240, 245}}
	}
	if w.C.Theme == "ember" {
		pal = [3][3]float64{{255, 115, 75}, {255, 200, 90}, {240, 80, 160}}
	}
	p := pal[species%3]
	v := clamp(brightness, 0, 1)
	c.R = math.Max(c.R, p[0]*v)
	c.G = math.Max(c.G, p[1]*v)
	c.B = math.Max(c.B, p[2]*v)
	c.Glow = math.Max(c.Glow, v)
	c.Glyph = glyph
}
func (w *World) paintBranches() {
	for _, b := range w.Branches {
		steps := int(math.Max(1, b.Length*2))
		for k := 0; k <= steps; k++ {
			t := float64(k) / float64(steps)
			x := int(b.X + math.Cos(b.Angle)*b.Length*t)
			y := int(b.Y + math.Sin(b.Angle)*b.Length*t)
			w.paint(x, y, .12+b.Energy*.13, 0, '·')
		}
	}
}
func (w *World) render(plain bool) string {
	var b strings.Builder
	if !plain {
		b.WriteString("\x1b[H")
	}
	for y := 0; y < w.C.Height; y++ {
		for x := 0; x < w.C.Width; x++ {
			c := w.Cells[y*w.C.Width+x]
			if c.Glow < .02 {
				b.WriteByte(' ')
				continue
			}
			glyph := c.Glyph
			if glyph == ' ' {
				glyph = '·'
			}
			if plain {
				b.WriteRune(glyph)
			} else {
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm%c", int(clamp(c.R, 0, 255)), int(clamp(c.G, 0, 255)), int(clamp(c.B, 0, 255)), glyph)
			}
		}
		if !plain {
			b.WriteString("\x1b[0m")
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "LUMINOUS GROVE  tick %d  organisms %d  branches %d  births %d  deaths %d\n", w.Tick, len(w.Agents), len(w.Branches), w.Births, w.Deaths)
	fmt.Fprintf(&b, "CPU %.1f%%  MEM %.1f%%  PSI %.2f  LOAD %.2f  PROCESSES %d  SEED %d\n", w.Metrics.CPU, w.Metrics.Memory, w.Metrics.Pressure, w.Metrics.Load, w.Metrics.Processes, w.C.Seed)
	return b.String()
}
func (w *World) snapshot() Snapshot {
	return Snapshot{Config: w.C, Tick: w.Tick, Timestamp: time.Now(), Metrics: w.Metrics, Agents: append([]Agent(nil), w.Agents...), Branches: append([]Branch(nil), w.Branches...)}
}
func writeFile(path string, bytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}
func saveJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return writeFile(path, append(b, '\n'))
}
func loadConfig(path string, c *Config) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, c)
}
func terminalSize() (int, int) {
	w, h := 100, 30
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			w = n
		}
	}
	if v := os.Getenv("LINES"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			h = n - 3
		}
	}
	return w, h
}
func main() {
	tw, th := terminalSize()
	c := Config{Width: tw, Height: th, Population: 85, FPS: 20, Mode: "live", Theme: "grove", Telemetry: true, Growth: 1, Trails: .91}
	configPath := flag.String("config", "", "JSON configuration path")
	width := flag.Int("width", 0, "canvas columns")
	height := flag.Int("height", 0, "canvas rows")
	population := flag.Int("population", -1, "initial organism population")
	fps := flag.Int("fps", 0, "frames per second")
	seed := flag.Int64("seed", 0, "random seed (zero selects time)")
	mode := flag.String("mode", "", "live, still, snapshot, inspect, or benchmark")
	theme := flag.String("theme", "", "grove, moon, ember")
	telemetry := flag.Bool("telemetry", true, "connect ecosystem to Linux metrics")
	growth := flag.Float64("growth", -1, "growth multiplier")
	trails := flag.Float64("trails", -1, "trail persistence 0.5-0.995")
	duration := flag.Int("duration", 0, "run duration in seconds (zero means unlimited in live mode)")
	output := flag.String("output", "", "output file path")
	csvPath := flag.String("csv", "", "optional CSV metrics recording")
	plain := flag.Bool("plain", false, "disable ANSI truecolor")
	flag.Parse()
	if *configPath != "" {
		if e := loadConfig(*configPath, &c); e != nil {
			fatal(e)
		}
	}
	if *width > 0 {
		c.Width = *width
	}
	if *height > 0 {
		c.Height = *height
	}
	if *population >= 0 {
		c.Population = *population
	}
	if *fps > 0 {
		c.FPS = *fps
	}
	if *seed != 0 {
		c.Seed = *seed
	}
	if *mode != "" {
		c.Mode = *mode
	}
	if *theme != "" {
		c.Theme = *theme
	}
	if *growth >= 0 {
		c.Growth = *growth
	}
	if *trails >= 0 {
		c.Trails = *trails
	}
	if *duration > 0 {
		c.Duration = *duration
	}
	if *output != "" {
		c.Output = *output
	}
	if !*telemetry {
		c.Telemetry = false
	}
	if c.Width < 20 || c.Width > 400 || c.Height < 8 || c.Height > 150 || c.Population < 0 || c.Population > 3000 || c.FPS < 1 || c.FPS > 120 {
		fatal(fmt.Errorf("invalid dimensions, population, or FPS"))
	}
	if c.Theme != "grove" && c.Theme != "moon" && c.Theme != "ember" {
		fatal(fmt.Errorf("unknown theme %q", c.Theme))
	}
	w := newWorld(c)
	w.sample()
	if c.Mode == "inspect" {
		fmt.Printf("Luminous Grove | Go %s | %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Canvas %dx%d | population %d | FPS %d | seed %d\n", c.Width, c.Height, c.Population, c.FPS, c.Seed)
		fmt.Printf("CPU %.1f%% | memory %.1f%% | PSI %.2f | processes %d\n", w.Metrics.CPU, w.Metrics.Memory, w.Metrics.Pressure, w.Metrics.Processes)
		return
	}
	if c.Mode != "live" && c.Mode != "still" && c.Mode != "snapshot" && c.Mode != "benchmark" {
		fatal(fmt.Errorf("unknown mode %q", c.Mode))
	}
	var csvFile *os.File
	var writer *csv.Writer
	if *csvPath != "" {
		var e error
		csvFile, e = os.Create(*csvPath)
		if e != nil {
			fatal(e)
		}
		defer csvFile.Close()
		writer = csv.NewWriter(csvFile)
		writer.Write([]string{"timestamp", "tick", "organisms", "branches", "cpu_percent", "memory_percent", "psi_avg10", "load", "processes"})
		defer writer.Flush()
	}
	record := func() {
		if writer != nil {
			m := w.Metrics
			writer.Write([]string{time.Now().Format(time.RFC3339), strconv.Itoa(w.Tick), strconv.Itoa(len(w.Agents)), strconv.Itoa(len(w.Branches)), fmt.Sprintf("%.3f", m.CPU), fmt.Sprintf("%.3f", m.Memory), fmt.Sprintf("%.3f", m.Pressure), fmt.Sprintf("%.3f", m.Load), strconv.Itoa(m.Processes)})
		}
	}
	if c.Mode == "still" || c.Mode == "snapshot" {
		for i := 0; i < c.FPS*8; i++ {
			w.update(1 / float64(c.FPS))
		}
		record()
		if c.Mode == "snapshot" {
			if c.Output == "" {
				c.Output = "grove-snapshot.json"
			}
			fatalIf(saveJSON(c.Output, w.snapshot()))
		} else {
			s := w.render(*plain)
			if c.Output != "" {
				fatalIf(writeFile(c.Output, []byte(s)))
			} else {
				fmt.Print(s)
			}
		}
		return
	}
	if c.Mode == "benchmark" {
		start := time.Now()
		for i := 0; i < c.FPS*10; i++ {
			w.update(1 / float64(c.FPS))
		}
		elapsed := time.Since(start)
		fmt.Printf("%d updates in %s (%.1f updates/s), %d organisms, %d branches\n", c.FPS*10, elapsed, float64(c.FPS*10)/elapsed.Seconds(), len(w.Agents), len(w.Branches))
		return
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	if !*plain {
		fmt.Print("\x1b[?25l\x1b[2J\x1b[H")
		defer fmt.Print("\x1b[0m\x1b[?25h\n")
	}
	ticker := time.NewTicker(time.Second / time.Duration(c.FPS))
	defer ticker.Stop()
	end := time.Time{}
	if c.Duration > 0 {
		end = time.Now().Add(time.Duration(c.Duration) * time.Second)
	}
	last := time.Now()
	frames := 0
	for {
		select {
		case <-sig:
			return
		case now := <-ticker.C:
			dt := clamp(now.Sub(last).Seconds(), .001, .2)
			last = now
			w.update(dt)
			fmt.Print(w.render(*plain))
			frames++
			if frames%c.FPS == 0 {
				record()
			}
			if !end.IsZero() && !now.Before(end) {
				if c.Output != "" {
					fatalIf(saveJSON(c.Output, w.snapshot()))
				}
				return
			}
		}
	}
}
func fatalIf(e error) {
	if e != nil {
		fatal(e)
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, "luminous-grove:", e); os.Exit(1) }

var _ = bufio.NewReader
