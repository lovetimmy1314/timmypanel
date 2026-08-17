#!/usr/bin/env python3
"""生成手机收藏 / 添加到主屏用的位图图标（apple-touch-icon、PWA manifest 图标）。

只用 Python 标准库：按 favicon/mark 的 SVG 几何手写光栅化（圆角矩形 + 4x 超采样），
因为构建机上不一定有 rsvg-convert / ImageMagick（brand/README.md 里那条命令需要 Linux 工具）。
改了标志几何（圆角、格子位置、色值）后重跑：

    python brand/gen_touch_icons.py

产物直接写进 frontend/public/icons/，vite build 会原样拷到 dist 根目录。
"""

import struct
import zlib
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "frontend" / "public" / "icons"

# 几何取自 brand/mark-light.svg（viewBox 128x128），色值见 brand/README.md。
# apple-touch-icon 由系统自己裁圆角，所以背景满铺，不留透明边。
VIEW = 128
BG = (0x53, 0x4A, 0xB7)
BG_R = 29.0
CELL = 26.0
CELL_R = 6.5
# (x, y, 颜色, 不透明度)，层序和 SVG 一致，后者盖前者。
CELLS = [
    (13, 13, (255, 255, 255), 1.0),
    (51, 13, (255, 255, 255), 1.0),
    (89, 13, (255, 255, 255), 1.0),
    (13, 51, (255, 255, 255), 0.22),
    (51, 51, (255, 255, 255), 1.0),
    (89, 51, (0x9F, 0xE1, 0xCB), 0.6),
    (13, 89, (255, 255, 255), 0.22),
    (51, 89, (255, 255, 255), 1.0),
    (89, 89, (255, 255, 255), 0.22),
]

SIZES = {"apple-touch-icon.png": 180, "icon-192.png": 192, "icon-512.png": 512}


def in_rrect(px: float, py: float, x: float, y: float, w: float, h: float, r: float) -> bool:
    """点 (px, py) 是否落在圆角矩形内（像素中心采样）。"""
    if px < x or px >= x + w or py < y or py >= y + h:
        return False
    cx = min(max(px, x + r), x + w - r)
    cy = min(max(py, y + r), y + h - r)
    return (px - cx) ** 2 + (py - cy) ** 2 <= r * r


def render(size: int, ss: int = 4) -> bytes:
    """超采样渲染再大盒式降采样，返回 RGBA 字节串。"""
    s = size * ss
    scale = s / VIEW
    hi = bytearray(s * s * 4)
    for j in range(s):
        py = (j + 0.5) / scale
        for i in range(s):
            px = (i + 0.5) / scale
            o = (j * s + i) * 4
            if not in_rrect(px, py, 0, 0, VIEW, VIEW, BG_R):
                continue  # 背景外的角保持透明
            r, g, b = BG
            for x, y, (cr, cg, cb), a in CELLS:
                if in_rrect(px, py, x, y, CELL, CELL, CELL_R):
                    r = cr * a + r * (1 - a)
                    g = cg * a + g * (1 - a)
                    b = cb * a + b * (1 - a)
            hi[o] = int(r + 0.5)
            hi[o + 1] = int(g + 0.5)
            hi[o + 2] = int(b + 0.5)
            hi[o + 3] = 255

    out = bytearray(size * size * 4)
    n = ss * ss
    for j in range(size):
        for i in range(size):
            acc = [0, 0, 0, 0]
            for dj in range(ss):
                base = ((j * ss + dj) * s + i * ss) * 4
                for di in range(ss):
                    o = base + di * 4
                    acc[0] += hi[o]
                    acc[1] += hi[o + 1]
                    acc[2] += hi[o + 2]
                    acc[3] += hi[o + 3]
            o = (j * size + i) * 4
            out[o] = acc[0] // n
            out[o + 1] = acc[1] // n
            out[o + 2] = acc[2] // n
            out[o + 3] = acc[3] // n
    return bytes(out)


def write_png(path: Path, size: int, rgba: bytes) -> None:
    def chunk(tag: bytes, data: bytes) -> bytes:
        return (
            struct.pack(">I", len(data))
            + tag
            + data
            + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)
        )

    rows = b"".join(
        b"\x00" + rgba[j * size * 4 : (j + 1) * size * 4] for j in range(size)
    )
    ihdr = struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0)
    path.write_bytes(
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", ihdr)
        + chunk(b"IDAT", zlib.compress(rows, 9))
        + chunk(b"IEND", b"")
    )


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for name, size in SIZES.items():
        write_png(OUT / name, size, render(size))
        print(f"写出 {OUT / name}")


if __name__ == "__main__":
    main()
