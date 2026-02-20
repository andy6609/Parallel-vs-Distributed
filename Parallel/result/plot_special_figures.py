"""
Special figures: Modulus vs If, pprof, Memory, Architecture Diagrams
"""

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import matplotlib.ticker as ticker
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch
import numpy as np
import os

OUT = os.path.join(os.path.dirname(__file__), "figures")
os.makedirs(OUT, exist_ok=True)

GRIDS = [128, 256, 512]

def save(name):
    path = os.path.join(OUT, name)
    plt.savefig(path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"  saved → {path}")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 11 – Ghost Row vs If-Statement Boundary Handling
# Parallel uses if-statements; Distributed uses ghost-row padding (halo exchange)
# ═══════════════════════════════════════════════════════════════════════════════
# 5-run averages (µs per full grid neighbour-count pass)
if_us = {
    128: np.mean([341.592, 340.408, 324.875, 319.533, 313.125]),
    256: np.mean([1344.567, 1460.892, 1402.333, 1367.808, 1317.417]),
    512: np.mean([4882.483, 5136.733, 5375.283, 5316.750, 5332.867]),
}
ghost_us = {
    128: np.mean([213.233, 217.183, 251.417, 231.933, 238.608]),
    256: np.mean([767.850, 789.333, 759.700, 761.708, 756.650]),
    512: np.mean([3807.925, 3121.725, 3399.792, 3331.417, 3295.717]),
}

fig, axes = plt.subplots(1, 2, figsize=(11, 4.5))

# Left: absolute time
ax = axes[0]
x = np.arange(len(GRIDS))
w = 0.35
if_vals    = [if_us[g]    / 1000 for g in GRIDS]   # → ms
ghost_vals = [ghost_us[g] / 1000 for g in GRIDS]
ax.bar(x - w/2, if_vals,    w, label="If-statement (Parallel)",     color="#ff7f0e", alpha=0.85)
ax.bar(x + w/2, ghost_vals, w, label="Ghost Row / Halo (Distributed)", color="#1f77b4", alpha=0.85)
for i, (f, g) in enumerate(zip(if_vals, ghost_vals)):
    ax.text(i - w/2, f + 0.05, f"{f:.2f}", ha="center", fontsize=7.5)
    ax.text(i + w/2, g + 0.05, f"{g:.2f}", ha="center", fontsize=7.5)
ax.set_xlabel("Grid Size")
ax.set_ylabel("Time per full-grid pass (ms)")
ax.set_title("Absolute Neighbour-Count Time")
ax.set_xticks(x)
ax.set_xticklabels([f"{g}×{g}" for g in GRIDS])
ax.legend(fontsize=8)
ax.grid(True, axis="y", alpha=0.3)

# Right: speedup of Ghost Row over If-statement
ax2 = axes[1]
ratios = [if_us[g] / ghost_us[g] for g in GRIDS]
bars = ax2.bar([f"{g}×{g}" for g in GRIDS], ratios,
               color=["#1f77b4", "#1f77b4", "#1f77b4"], alpha=0.85)
ax2.axhline(1.0, color="black", linestyle="--", linewidth=0.9, label="Baseline (1×)")
for bar, r in zip(bars, ratios):
    ax2.text(bar.get_x() + bar.get_width()/2, r + 0.02, f"{r:.2f}×",
             ha="center", fontsize=10, fontweight="bold")
ax2.set_xlabel("Grid Size")
ax2.set_ylabel("Speedup of Ghost Row over If-statement")
ax2.set_title("Relative Performance\n(Ghost Row speedup, arm64 M4)")
ax2.set_ylim(0, 2.2)
ax2.legend(fontsize=9)
ax2.grid(True, axis="y", alpha=0.3)

fig.suptitle("Boundary Handling: If-Statement (Parallel) vs Ghost Row (Distributed)", fontsize=12)
fig.tight_layout()
save("fig11_ghostrow_vs_if.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 12 – pprof CPU Usage (top functions, 512×512 t4, 1000 turns)
# ═══════════════════════════════════════════════════════════════════════════════
pprof_funcs = [
    ("countAliveNeighbors",   40.68),
    ("pthread_cond_signal\n(goroutine sync)",  27.86),
    ("pthread_cond_wait\n(goroutine sync)",     6.32),
    ("runtime.usleep",         5.08),
    ("applyRules",             4.63),
    ("processTask (cum)",      4.34),
    ("other",                  10.09),
]
labels = [f[0] for f in pprof_funcs]
pcts   = [f[1] for f in pprof_funcs]
colors = ["#d62728","#ff7f0e","#ffbb78","#aec7e8","#1f77b4","#2ca02c","#cccccc"]

fig, ax = plt.subplots(figsize=(8, 5))
bars = ax.barh(labels[::-1], pcts[::-1], color=colors[::-1], alpha=0.88)
for bar, pct in zip(bars, pcts[::-1]):
    ax.text(bar.get_width() + 0.4, bar.get_y() + bar.get_height()/2,
            f"{pct:.2f}%", va="center", fontsize=9)
ax.set_xlabel("CPU Time (%)")
ax.set_title("Top CPU Functions – pprof (512×512, t4, 1000 turns)\n"
             "Total samples = 24.19s (261% across 4 goroutines)")
ax.set_xlim(0, 50)
ax.grid(True, axis="x", alpha=0.3)
fig.tight_layout()
save("fig12_pprof_cpu.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 13 – Memory Allocation: Worker Pool vs Serial (benchmem)
# ═══════════════════════════════════════════════════════════════════════════════
mem_data = {
    # (MB, allocs/op) – 1x run, 1000 turns
    "Serial 128²":     (20.8,   140160),
    "Serial 256²":     (127.1,  312435),
    "Serial 512²":     (473.8,  537597),
    "Parallel-t1 128²":(40.3,   269204),
    "Parallel-t4 128²":(40.6,   272311),
    "Parallel-t1 512²":(749.5, 1051950),
    "Parallel-t4 512²":(705.9, 1090247),
    "Parallel-t16 512²":(710.2,1214777),
}

fig, axes = plt.subplots(1, 2, figsize=(12, 5))

# Left: MB allocated
categories = list(mem_data.keys())
mb_vals  = [mem_data[k][0] for k in categories]
alloc_vals=[mem_data[k][1] for k in categories]
colors_mem = ["#9467bd"]*3 + ["#1f77b4","#2ca02c","#ff7f0e","#d62728","#8c564b"]

ax = axes[0]
bars = ax.barh(categories, mb_vals, color=colors_mem, alpha=0.85)
for bar, v in zip(bars, mb_vals):
    ax.text(bar.get_width() + 2, bar.get_y() + bar.get_height()/2,
            f"{v:.0f} MB", va="center", fontsize=8)
ax.set_xlabel("Memory Allocated (MB)")
ax.set_title("Total Memory Allocated\n(1000 turns, 1x run)")
ax.grid(True, axis="x", alpha=0.3)
ax.set_xlim(0, 870)

# Right: allocs/op
ax2 = axes[1]
bars2 = ax2.barh(categories, [a/1000 for a in alloc_vals], color=colors_mem, alpha=0.85)
for bar, v in zip(bars2, alloc_vals):
    ax2.text(bar.get_width() + 5, bar.get_y() + bar.get_height()/2,
             f"{v/1000:.0f}K", va="center", fontsize=8)
ax2.set_xlabel("Allocations (thousands)")
ax2.set_title("Number of Heap Allocations\n(1000 turns, 1x run)")
ax2.grid(True, axis="x", alpha=0.3)

serial_patch = mpatches.Patch(color="#9467bd", alpha=0.85, label="Serial (no Worker Pool)")
parallel_patch = mpatches.Patch(color="#1f77b4", alpha=0.85, label="Parallel (Worker Pool)")
fig.legend(handles=[serial_patch, parallel_patch], loc="lower center",
           ncol=2, bbox_to_anchor=(0.5, -0.05))
fig.suptitle("Memory Usage: Serial vs Worker Pool Parallel", fontsize=12)
fig.tight_layout()
save("fig13_memory_allocation.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 14 – Architecture: Parallel Implementation (Goroutine Interaction)
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(12, 6))
ax.set_xlim(0, 12)
ax.set_ylim(0, 6)
ax.axis("off")
ax.set_facecolor("#f8f9fa")
fig.patch.set_facecolor("#f8f9fa")

def box(ax, x, y, w, h, label, color="#dce8f7", fontsize=9, bold=False):
    rect = FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.05",
                          linewidth=1.2, edgecolor="#555", facecolor=color)
    ax.add_patch(rect)
    weight = "bold" if bold else "normal"
    ax.text(x + w/2, y + h/2, label, ha="center", va="center",
            fontsize=fontsize, fontweight=weight, wrap=True,
            multialignment="center")

def arrow(ax, x1, y1, x2, y2, label="", color="#333"):
    ax.annotate("", xy=(x2, y2), xytext=(x1, y1),
                arrowprops=dict(arrowstyle="->", color=color, lw=1.4))
    if label:
        mx, my = (x1+x2)/2, (y1+y2)/2
        ax.text(mx, my + 0.12, label, ha="center", fontsize=7.5, color=color)

# Main outer boxes
box(ax, 0.2, 0.3, 3.5, 5.3, "", color="#e8f4e8")
ax.text(1.95, 5.4, "Local Machine", ha="center", fontsize=10, fontweight="bold", color="#2d6a2d")

box(ax, 4.0, 1.8, 3.6, 3.5, "", color="#fff3e0")
ax.text(5.8, 5.1, "Worker Pool", ha="center", fontsize=10, fontweight="bold", color="#8a4500")

# Main goroutine components
box(ax, 0.4, 4.2, 1.5, 1.0, "main()\ngol.Run()", color="#c8e6c9", fontsize=8)
box(ax, 0.4, 2.9, 1.5, 1.0, "distributor\ngoroutine", color="#bbdefb", fontsize=8)
box(ax, 0.4, 1.6, 1.5, 1.0, "IO\ngoroutine", color="#e1bee7", fontsize=8)
box(ax, 0.4, 0.4, 1.5, 1.0, "SDL / Events\nchannel", color="#ffe0b2", fontsize=8)
box(ax, 2.1, 2.9, 1.4, 1.0, "keyPresses\nchan rune", color="#f5f5f5", fontsize=8)

# Worker pool internals
for i, label in enumerate(["Worker\n#1", "Worker\n#2", "Worker\n#N"]):
    ypos = 4.0 - i * 1.05
    box(ax, 4.2, ypos, 1.1, 0.85, label, color="#ffe082", fontsize=8)
    if i == 1:
        ax.text(4.75, 2.75, "…", ha="center", fontsize=14)

box(ax, 5.6, 2.0, 1.8, 1.0, "taskChan\n(buffered)", color="#f5f5f5", fontsize=8)
box(ax, 5.6, 3.2, 1.8, 1.0, "resultChan\n(buffered)", color="#f5f5f5", fontsize=8)

# Result aggregator
box(ax, 7.8, 2.5, 1.8, 1.2, "Result\nAggregator\n(main thread)", color="#c8e6c9", fontsize=8)

# Events output
box(ax, 7.8, 4.2, 1.8, 1.0, "Events chan\n(FinalTurn\nTurnComplete…)", color="#ffe0b2", fontsize=8)

# Arrows
arrow(ax, 1.15, 4.2, 1.15, 3.9)                        # main→distributor
arrow(ax, 1.15, 2.9, 1.15, 2.6)                        # distributor→IO
arrow(ax, 1.55, 2.9+0.5, 4.2, 3.5, "tasks", "#555")   # dist→worker pool
arrow(ax, 5.6+0.9, 3.2, 7.8, 3.1, "results", "#555")  # resultChan→aggregator
arrow(ax, 5.6, 2.5, 4.2+1.1, 2.5)                      # taskChan←worker
arrow(ax, 4.2+1.1, 3.5, 5.6, 3.5)                      # worker→resultChan
arrow(ax, 9.6, 4.7, 11.2, 4.7, "Events", "#8a4500")
arrow(ax, 9.6, 3.1, 9.6, 4.2)                          # aggregator→events

ax.text(5.8, 5.7, "Goroutine Interaction Diagram – Parallel GoL",
        ha="center", fontsize=13, fontweight="bold")
fig.tight_layout()
save("fig14_arch_parallel.png")


# ═══════════════════════════════════════════════════════════════════════════════
# Figure 15 – Architecture: Distributed Implementation
# ═══════════════════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(14, 6))
ax.set_xlim(0, 14)
ax.set_ylim(0, 6.5)
ax.axis("off")
ax.set_facecolor("#f8f9fa")
fig.patch.set_facecolor("#f8f9fa")

# ── Local Controller ────────────────────────────────────────────────────────
box(ax, 0.2, 0.3, 4.4, 5.8, "", color="#e8f4e8")
ax.text(2.4, 5.9, "Local Controller", ha="center", fontsize=11, fontweight="bold", color="#2d6a2d")

box(ax, 0.4, 4.6, 1.8, 1.0, "Keypresses\nchan rune", color="#c8e6c9", fontsize=8)
box(ax, 2.4, 4.6, 1.8, 1.0, "Ticker\n(2s alive count)", color="#c8e6c9", fontsize=8)
box(ax, 0.4, 3.3, 3.8, 1.0, "distributor\n(setupRPC → startGame → event loop)", color="#bbdefb", fontsize=8)
box(ax, 0.4, 2.1, 1.5, 1.0, "IO goroutine\n(PGM in/out)", color="#e1bee7", fontsize=8)
box(ax, 2.1, 2.1, 2.1, 1.0, "Filename / Output\n/ Input / Idle chans", color="#f5f5f5", fontsize=8)
box(ax, 0.4, 0.5, 1.5, 1.0, "SDL\nGraphics", color="#ffe0b2", fontsize=8)
box(ax, 2.1, 0.5, 2.1, 1.0, "Events chan\n(FinalTurn\nStateChange…)", color="#ffe0b2", fontsize=8)

# ── Broker ──────────────────────────────────────────────────────────────────
box(ax, 5.0, 0.3, 4.0, 5.8, "", color="#fff3e0")
ax.text(7.0, 5.9, "Broker (EC2)", ha="center", fontsize=11, fontweight="bold", color="#8a4500")

box(ax, 5.2, 4.2, 3.6, 1.2, "Process() / runSimulation()\n(s *Server, RPC listener :8030)", color="#ffe082", fontsize=8)
box(ax, 5.2, 2.8, 3.6, 1.1, "distributeWork()\nactivePartitions()\nbuildWorkerSlice()", color="#ffcc80", fontsize=8)
box(ax, 5.2, 1.5, 1.6, 1.1, "GetWorld()\nGetAliveCells()", color="#ffe0b2", fontsize=8)
box(ax, 7.0, 1.5, 1.8, 1.1, "Pause / Resume\nShutdown / Stop", color="#ffe0b2", fontsize=8)
box(ax, 5.2, 0.5, 3.6, 0.8, "reconnectWorker()  •  fault tolerance", color="#ffb74d", fontsize=8)

# ── AWS NODE ────────────────────────────────────────────────────────────────
box(ax, 9.4, 0.3, 4.3, 5.8, "", color="#fce4ec")
ax.text(11.55, 5.9, "AWS Node (EC2)", ha="center", fontsize=11, fontweight="bold", color="#880e4f")

for i, (port, label) in enumerate([("8031","Worker 1"), ("8032","Worker 2")]):
    y = 3.5 - i * 2.3
    box(ax, 9.6, y, 3.9, 2.0, f"GolWorker ({port})\n\nCalculateNextState()\nGhost-row padding\napplyRules(Conway/HighLife/D&N)", color="#f8bbd0", fontsize=7.5)

# ── Arrows ──────────────────────────────────────────────────────────────────
# Local → Broker
ax.annotate("", xy=(5.0, 3.8), xytext=(4.6, 3.8),
            arrowprops=dict(arrowstyle="<->", color="#1565c0", lw=2.0))
ax.text(4.8, 4.05, "RPC Call\n:8030", ha="center", fontsize=8, color="#1565c0", fontweight="bold")

# Broker → Workers
ax.annotate("", xy=(9.4, 4.3), xytext=(9.0, 4.3),
            arrowprops=dict(arrowstyle="<->", color="#880e4f", lw=2.0))
ax.text(9.2, 4.55, "RPC Call\n(private IP)", ha="center", fontsize=8, color="#880e4f", fontweight="bold")
ax.annotate("", xy=(9.4, 2.0), xytext=(9.0, 2.0),
            arrowprops=dict(arrowstyle="<->", color="#880e4f", lw=2.0))

ax.text(7.0, 6.3, "Distributed GoL Architecture – Broker/Worker via RPC (AWS EC2)",
        ha="center", fontsize=13, fontweight="bold")
fig.tight_layout()
save("fig15_arch_distributed.png")

print("\nAll special figures saved to:", OUT)
