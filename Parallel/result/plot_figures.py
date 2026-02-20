"""
Conway's Game of Life – Benchmark Figure Generator
Parallel & Distributed: measured (1000 turns, Apple M4 / AWS EC2)
"""

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker
import numpy as np
import os

OUT = os.path.join(os.path.dirname(__file__), "figures")
os.makedirs(OUT, exist_ok=True)

# ── colour palette (colorblind-friendly) ──────────────────────────────────────
C = {
    "serial":    "#444444",
    "128":       "#1f77b4",
    "256":       "#ff7f0e",
    "512":       "#2ca02c",
    "local":     "#9467bd",
    "aws":       "#d62728",
    "ideal":     "#aaaaaa",
}

THREADS = [1, 2, 4, 8, 16]

# ── raw data (ms, 1000 turns) ──────────────────────────────────────────────────
data = {
    128: {
        "serial": 415.1,
        "parallel": [394.9, 219.7, 152.5, 151.8, 142.5],   # t1..t16
        "local_dist_2w": 286,
        "aws_dist_2w":   439,
    },
    256: {
        "serial": 1628.4,
        "parallel": [1535.9, 838.5, 576.2, 499.8, 433.1],
        "local_dist_2w": 1090,
        "aws_dist_2w":   1929,
    },
    512: {
        "serial": 6318.0,
        "parallel": [6098.5, 3300.4, 2195.3, 1687.8, 1543.2],
        "local_dist_2w": 4291,
        "aws_dist_2w":   9901,
    },
}

GRIDS = [128, 256, 512]
STYLE = dict(linewidth=1.8, marker="o", markersize=5)


# ── helper ────────────────────────────────────────────────────────────────────
def save(name):
    path = os.path.join(OUT, name)
    plt.savefig(path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"  saved → {path}")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 1 – Thread Scaling: time vs thread count
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(7, 4.5))
for g in GRIDS:
    times_s = [t / 1000 for t in data[g]["parallel"]]
    ax.plot(THREADS, times_s, label=f"{g}×{g}", color=C[str(g)], **STYLE)
    # serial baseline as dashed horizontal
    ax.axhline(data[g]["serial"] / 1000, color=C[str(g)], linestyle="--",
               linewidth=0.9, alpha=0.5)

ax.set_xlabel("Number of Goroutine Workers")
ax.set_ylabel("Elapsed Time (s)")
ax.set_title("Thread Scaling – Parallel GoL (1000 turns)")
ax.set_xticks(THREADS)
ax.legend(title="Grid size", framealpha=0.85)
ax.yaxis.set_minor_locator(ticker.AutoMinorLocator())
ax.grid(True, which="major", alpha=0.3)
ax.grid(True, which="minor", alpha=0.1)
fig.tight_layout()
save("fig1_thread_scaling.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 2 – Speedup vs Serial (Amdahl's Law)
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(7, 4.5))
ideal_x = np.linspace(1, 16, 100)
ax.plot(ideal_x, ideal_x, color=C["ideal"], linestyle="--",
        linewidth=1.2, label="Ideal (linear)")

for g in GRIDS:
    serial = data[g]["serial"]
    speedups = [serial / t for t in data[g]["parallel"]]
    ax.plot(THREADS, speedups, label=f"{g}×{g}", color=C[str(g)], **STYLE)

ax.set_xlabel("Number of Goroutine Workers")
ax.set_ylabel("Speedup (×  over Serial)")
ax.set_title("Parallel Speedup vs Serial Baseline (1000 turns)")
ax.set_xticks(THREADS)
ax.set_xlim(0.5, 17)
ax.set_ylim(0)
ax.legend(framealpha=0.85)
ax.grid(True, alpha=0.3)
fig.tight_layout()
save("fig2_speedup.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 3 – Parallel vs Distributed (grouped bar)
# ═══════════════════════════════════════════════════════════════════════════════
labels = ["Serial", "Parallel t4", "Local Dist\n(2 workers)", "AWS Dist\n(2 workers)"]
x = np.arange(len(GRIDS))
width = 0.18
offsets = [-1.5, -0.5, 0.5, 1.5]
colors  = [C["serial"], C["128"], C["local"], C["aws"]]

fig, ax = plt.subplots(figsize=(8, 5))
for i, (label, off, col) in enumerate(zip(labels, offsets, colors)):
    vals = []
    for g in GRIDS:
        if label == "Serial":
            vals.append(data[g]["serial"] / 1000)
        elif label == "Parallel t4":
            vals.append(data[g]["parallel"][2] / 1000)   # index 2 = t4
        elif "Local" in label:
            vals.append(data[g]["local_dist_2w"] / 1000)
        else:
            vals.append(data[g]["aws_dist_2w"] / 1000)
    bars = ax.bar(x + off * width, vals, width, label=label, color=col, alpha=0.85)

ax.set_xlabel("Grid Size")
ax.set_ylabel("Elapsed Time (s)")
ax.set_title("Parallel vs Distributed Performance (1000 turns)")
ax.set_xticks(x)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.legend(framealpha=0.85)
ax.grid(True, axis="y", alpha=0.3)
fig.tight_layout()
save("fig3_parallel_vs_distributed.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 4 – Grid Size Scaling (log–log)
# ═══════════════════════════════════════════════════════════════════════════════
grid_px = [g * g for g in GRIDS]   # total cells (n²)

series = {
    "Serial":                ([data[g]["serial"]       for g in GRIDS], C["serial"],  "--"),
    "Parallel t16 (best)":   ([data[g]["parallel"][-1] for g in GRIDS], C["512"],    "-"),
    "Local Dist (2w)":       ([data[g]["local_dist_2w"] for g in GRIDS], C["local"], "-"),
    "AWS Dist (2w)":         ([data[g]["aws_dist_2w"]  for g in GRIDS], C["aws"],    "-"),
}

fig, ax = plt.subplots(figsize=(7, 4.5))
for label, (vals, col, ls) in series.items():
    vals_s = [v / 1000 for v in vals]
    ax.plot(grid_px, vals_s, color=col, linestyle=ls,
            marker="o", markersize=6, linewidth=1.8, label=label)

ax.set_xscale("log")
ax.set_yscale("log")
ax.set_xlabel("Grid Cells (n²)")
ax.set_ylabel("Elapsed Time (s)")
ax.set_title("Grid Size Scaling – Serial / Parallel / Distributed (1000 turns)")
ax.set_xticks(grid_px)
ax.set_xticklabels([f"{g}×{g}\n({g*g:,})" for g in GRIDS])
ax.legend(framealpha=0.85)
ax.grid(True, which="both", alpha=0.25)
fig.tight_layout()
save("fig4_grid_scaling.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 5 – Distributed Overhead Ratio (AWS / Parallel t2)
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(6, 4))
ratios_local = [data[g]["local_dist_2w"] / data[g]["parallel"][1] for g in GRIDS]
ratios_aws   = [data[g]["aws_dist_2w"]   / data[g]["parallel"][1] for g in GRIDS]

x = np.arange(len(GRIDS))
width = 0.35
ax.bar(x - width/2, ratios_local, width, label="Local Dist / Parallel t2",
       color=C["local"], alpha=0.85)
ax.bar(x + width/2, ratios_aws,   width, label="AWS Dist / Parallel t2",
       color=C["aws"],   alpha=0.85)

ax.axhline(1.0, color="black", linestyle="--", linewidth=0.9, label="Baseline (1×)")
ax.set_xlabel("Grid Size")
ax.set_ylabel("Overhead Ratio (×)")
ax.set_title("Distributed Overhead vs Parallel t2 Baseline")
ax.set_xticks(x)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.legend(framealpha=0.85)
ax.grid(True, axis="y", alpha=0.3)
fig.tight_layout()
save("fig5_distributed_overhead.png")

print("\nAll figures saved to:", OUT)
