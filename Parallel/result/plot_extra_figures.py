"""
Conway's Game of Life – Additional Benchmark Figures
"""

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker
import numpy as np
import os

OUT = os.path.join(os.path.dirname(__file__), "figures")
os.makedirs(OUT, exist_ok=True)

C = {
    "serial":      "#444444",
    "128":         "#1f77b4",
    "256":         "#ff7f0e",
    "512":         "#2ca02c",
    "conway":      "#1f77b4",
    "highlife":    "#ff7f0e",
    "daynight":    "#d62728",
    "local":       "#9467bd",
    "aws":         "#d62728",
    "ideal":       "#aaaaaa",
    "t1":          "#e377c2",
    "overhead":    "#8c564b",
}

GRIDS = [128, 256, 512]
STYLE = dict(linewidth=1.8, marker="o", markersize=5)


def save(name):
    path = os.path.join(OUT, name)
    plt.savefig(path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"  saved → {path}")


# ── Rule set data (5-run avg, 1000 turns, 4 threads, ms) ──────────────────────
ruleset_time = {
    "Conway":     {128: 186.1, 256: 588.7, 512: 2326.7},
    "HighLife":   {128: 150.3, 256: 639.2, 512: 2176.4},
    "DayAndNight":{128: 187.5, 256: 626.9, 512: 2302.9},
}

# ── Alive cells per rule set (100 turns, measured) ────────────────────────────
alive_cells = {
    "Conway":     {128: 525,   256: 2043,  512: 6645},
    "HighLife":   {128: 1637,  256: 6729,  512: 6278},
    "DayAndNight":{128: 11541, 256: 46715, 512: 1152},
}

# ── Parallel data (1000 turns, ms) ────────────────────────────────────────────
parallel = {
    128: {"serial": 415.1, "t1": 394.9, "t2": 219.7, "t4": 152.5, "t8": 151.8, "t16": 142.5},
    256: {"serial": 1628.4,"t1": 1535.9,"t2": 838.5, "t4": 576.2, "t8": 499.8, "t16": 433.1},
    512: {"serial": 6318.0,"t1": 6098.5,"t2": 3300.4,"t4": 2195.3,"t8": 1687.8,"t16": 1543.2},
}

# ── Distributed (ms) ──────────────────────────────────────────────────────────
dist = {
    128: {"local": 286,  "aws": 439},
    256: {"local": 1090, "aws": 1929},
    512: {"local": 4291, "aws": 9901},
}


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 6 – Rule Set Performance (time vs grid size)
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(7, 4.5))
rs_colors = {"Conway": C["conway"], "HighLife": C["highlife"], "DayAndNight": C["daynight"]}
rs_labels = {"Conway": "Conway (B3/S23)",
             "HighLife": "HighLife (B36/S23)",
             "DayAndNight": "Day & Night (B3678/S34678)"}

for rs, col in rs_colors.items():
    times = [ruleset_time[rs][g] / 1000 for g in GRIDS]
    ax.plot(GRIDS, times, label=rs_labels[rs], color=col, **STYLE)

ax.set_xlabel("Grid Size (n)")
ax.set_ylabel("Elapsed Time (s)")
ax.set_title("Rule Set Performance Comparison\n(4 threads, 1000 turns)")
ax.set_xticks(GRIDS)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.legend(framealpha=0.85)
ax.grid(True, alpha=0.3)
fig.tight_layout()
save("fig6_ruleset_performance.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 7 – Alive Cells per Rule Set (bar chart)
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(8, 5))
x = np.arange(len(GRIDS))
width = 0.26
offsets = [-1, 0, 1]
rulesets = ["Conway", "HighLife", "DayAndNight"]

for rs, off, col in zip(rulesets, offsets, rs_colors.values()):
    vals = [alive_cells[rs][g] for g in GRIDS]
    ax.bar(x + off * width, vals, width, label=rs_labels[rs], color=col, alpha=0.85)

ax.set_xlabel("Grid Size")
ax.set_ylabel("Alive Cells (after 100 turns)")
ax.set_title("Alive Cell Count by Rule Set (100 turns)\n"
             "Rule complexity shapes long-term population density")
ax.set_xticks(x)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.legend(framealpha=0.85)
ax.yaxis.set_major_formatter(ticker.FuncFormatter(lambda v, _: f"{int(v):,}"))
ax.grid(True, axis="y", alpha=0.3)
fig.tight_layout()
save("fig7_alive_cells_ruleset.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 8 – Serial vs Parallel-t1 (Worker Pool overhead)
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(7, 4.5))
serial_times = [parallel[g]["serial"] / 1000 for g in GRIDS]
t1_times     = [parallel[g]["t1"]     / 1000 for g in GRIDS]

ax.plot(GRIDS, serial_times, label="Serial (no Worker Pool)", color=C["serial"],
        linestyle="--", **STYLE)
ax.plot(GRIDS, t1_times,     label="Parallel t1 (Worker Pool, 1 worker)",
        color=C["t1"], **STYLE)

for g, s, t in zip(GRIDS, serial_times, t1_times):
    overhead_pct = (t - s) / s * 100
    ax.annotate(f"{overhead_pct:+.1f}%", xy=(g, t),
                xytext=(0, 8), textcoords="offset points",
                ha="center", fontsize=8, color=C["t1"])

ax.set_xlabel("Grid Size (n)")
ax.set_ylabel("Elapsed Time (s)")
ax.set_title("Worker Pool Overhead: Serial vs Parallel t1\n(1000 turns)")
ax.set_xticks(GRIDS)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.legend(framealpha=0.85)
ax.grid(True, alpha=0.3)
fig.tight_layout()
save("fig8_serial_vs_t1_overhead.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 9 – Local vs AWS by grid size
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(7, 4.5))

best_parallel = [parallel[g]["t16"] / 1000 for g in GRIDS]
local_dist    = [dist[g]["local"]   / 1000 for g in GRIDS]
aws_dist      = [dist[g]["aws"]     / 1000 for g in GRIDS]

ax.plot(GRIDS, best_parallel, label="Parallel t16 (local best)",
        color=C["512"], **STYLE)
ax.plot(GRIDS, local_dist,    label="Local Distributed (2 workers)",
        color=C["local"], linestyle="--", **STYLE)
ax.plot(GRIDS, aws_dist,      label="AWS Distributed (2 workers)",
        color=C["aws"],   linestyle="--", **STYLE)

ax.set_xlabel("Grid Size (n)")
ax.set_ylabel("Elapsed Time (s)")
ax.set_title("Local Parallel vs Distributed Performance by Grid Size")
ax.set_xticks(GRIDS)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.legend(framealpha=0.85)
ax.grid(True, alpha=0.3)
fig.tight_layout()
save("fig9_local_vs_aws_by_grid.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 10 – Rule Set Normalised Performance (relative to Conway)
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(7, 4.5))

for rs, col in rs_colors.items():
    rel = [ruleset_time[rs][g] / ruleset_time["Conway"][g] for g in GRIDS]
    ax.plot(GRIDS, rel, label=rs_labels[rs], color=col, **STYLE)

ax.axhline(1.0, color="black", linestyle="--", linewidth=0.9, label="Conway baseline (1×)")
ax.set_xlabel("Grid Size (n)")
ax.set_ylabel("Relative Time (Conway = 1×)")
ax.set_title("Rule Set Performance Relative to Conway\n(4 threads, 1000 turns)")
ax.set_xticks(GRIDS)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.set_ylim(0.5, 1.6)
ax.legend(framealpha=0.85)
ax.grid(True, alpha=0.3)
fig.tight_layout()
save("fig10_ruleset_relative.png")


print("\nAll extra figures saved to:", OUT)
