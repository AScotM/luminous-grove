# Luminous Grove

Luminous Grove is a self-contained, terminal-native generative ecosystem for Linux, written entirely in the Go standard library. It translates system activity into a nocturnal world of drifting luminous organisms, branching vegetation, and fading phosphorescent trails.

The project is designed for Fedora Server 45 and other Linux servers. No graphical desktop, external Go modules, or elevated privileges are required.

## Build

```sh
sudo dnf install -y golang
cd luminous-grove
go build -trimpath -o luminous-grove main.go
./luminous-grove
```

Press Ctrl+C to stop and restore the cursor.

## Modes

```sh
./luminous-grove -mode live -theme moon -population 140 -fps 24
./luminous-grove -mode live -theme ember -duration 30 -output final-state.json
./luminous-grove -mode still -plain -output grove.txt
./luminous-grove -mode snapshot -output grove.json
./luminous-grove -mode inspect
./luminous-grove -mode benchmark
```

`live` renders continuously. `still` advances the ecosystem for eight simulated seconds and renders a single frame. `snapshot` writes a structured state after eight simulated seconds. `inspect` reports the current runtime and Linux telemetry. `benchmark` measures simulation throughput without terminal rendering.

## Options

| Flag | Purpose | Default |
| --- | --- | --- |
| `-width` | Canvas width, 20–400 | `COLUMNS` or 100 |
| `-height` | Canvas height, 8–150 | `LINES - 3` or 30 |
| `-population` | Initial organisms, 0–3000 | 85 |
| `-fps` | Simulation/render rate, 1–120 | 20 |
| `-seed` | Deterministic random seed | Clock-derived |
| `-mode` | `live`, `still`, `snapshot`, `inspect`, `benchmark` | `live` |
| `-theme` | `grove`, `moon`, `ember` | `grove` |
| `-telemetry` | Couple activity to Linux metrics | true |
| `-growth` | Vegetation and population growth rate | 1 |
| `-trails` | Phosphor persistence, 0.5–0.995 | 0.91 |
| `-duration` | Limit live session in seconds | unlimited |
| `-output` | Save final JSON state or still frame | stdout / none |
| `-csv` | Record metrics once per second | none |
| `-plain` | Disable ANSI color and cursor controls | false |
| `-config` | Load JSON configuration | none |

## Example: observatory session

```sh
./luminous-grove -width 110 -height 32 -population 180 -theme grove -fps 20 -trails 0.95 -csv metrics.csv -duration 120 -output session.json
```

A JSON configuration file uses the same names as the fields in `Config`, lowercase (`width`, `height`, `population`, `fps`, `seed`, `mode`, `theme`, `telemetry`, `growth`, `trails`, `duration_seconds`, `output`). CLI flags explicitly set to non-default override the file. For disabling telemetry, pass `-telemetry=false` without a config file.

## Simulation

Each organism has position, velocity, energy, age, phase, and one of three species. Local interactions combine repulsion at close range, weak same-species attraction, procedural oscillation, and noise. Organisms reflect from the canvas boundaries. Energy slowly depletes; expired organisms are removed and replacements emerge according to the configured growth rate.

Vegetation starts from the bottom of the canvas. Each branch can spawn new angled branches until a depth limit or canvas boundary is reached. Branching is stochastic and limited to keep memory bounded.

A persistent RGB canvas creates fading trails. Three palettes are provided: forest-green `grove`, cool blue `moon`, and warm `ember`.

## Linux telemetry

Luminous Grove reads `/proc/stat`, `/proc/meminfo`, `/proc/pressure/cpu`, `/proc/loadavg`, and the number of numeric directories under `/proc`. CPU utilization is derived from consecutive aggregate counter samples. CPU activity affects organism velocity and target population. Other metrics are displayed in the status footer and available in exported records.

No telemetry is transmitted over the network. Telemetry is best-effort: unavailable procfs files result in zero-valued metrics. This is a visualization, not a precision monitoring or alerting system.

## Recording and reproducibility

`-csv metrics.csv` records a metrics timeline. `-mode snapshot` and the timed live mode's `-output` produce JSON containing the configuration, tick, Linux metrics, agents, and branches. `-seed` fixes the random sequence for controlled experiments, although live timing and changing host telemetry still affect results.

For reproducible experiments, disable telemetry and use a fixed seed:

```sh
./luminous-grove -mode snapshot -seed 2026 -telemetry=false -output reproducible.json
```

## Performance

Neighbor interaction is quadratic in the number of organisms. Use 50–200 organisms for smooth terminal animation, and benchmark before choosing larger populations. Rendering many individually colored characters may be expensive on slow remote terminals. Reduce `-fps`, dimensions, and population for SSH sessions.

## Security and operational notes

Runs as an unprivileged user. Reads local kernel virtual files only. Writes output files only when requested. Use Ctrl+C to restore the terminal cursor. Avoid redirecting live ANSI output to logs; use `still`, `snapshot`, or CSV recording instead.

## License

GNU General Public License version 3.0 or later. See `LICENSE`.
