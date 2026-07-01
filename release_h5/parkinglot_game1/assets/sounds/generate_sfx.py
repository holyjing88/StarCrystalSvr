#!/usr/bin/env python3
"""
生成标准 RIFF PCM int16 立体声 WAV（44100Hz），兼容播放器与 Web Audio。
此前部分 Mixkit 下发为 WAVEFORMATEXTENSIBLE 24-bit，部分软件会静音。
"""
from __future__ import annotations

import math
import random
import struct
import wave
from pathlib import Path

SR = 44100


def clip_i16(x: float) -> int:
    v = int(round(x * 32767.0))
    return max(-32768, min(32767, v))


def write_stereo_wav(path: Path, frames_lr: list[tuple[float, float]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with wave.open(str(path), "wb") as w:
        w.setnchannels(2)
        w.setsampwidth(2)
        w.setframerate(SR)
        for lo, ro in frames_lr:
            w.writeframes(struct.pack("<hh", clip_i16(lo), clip_i16(ro)))


def env_exp(i: int, n: int, k: float = 10.0) -> float:
    if n <= 0:
        return 0.0
    return math.exp(-k * i / n)


def gen_shoot() -> list[tuple[float, float]]:
    n = int(SR * 0.065)
    out: list[tuple[float, float]] = []
    rng = random.Random(42)
    for i in range(n):
        t = i / SR
        e = env_exp(i, n, 14.0)
        noise = (rng.random() * 2.0 - 1.0) * 0.72
        chirp = math.sin(2.0 * math.pi * (2200.0 - 9000.0 * t) * t) * 0.38
        s = (noise * 0.62 + chirp) * e * 0.92
        out.append((s, s))
    return out


def gen_hit() -> list[tuple[float, float]]:
    n = int(SR * 0.11)
    out: list[tuple[float, float]] = []
    rng = random.Random(7)
    f1, f2 = 620.0, 155.0
    for i in range(n):
        t = i / SR
        e = env_exp(i, n, 11.0)
        body = (
            math.sin(2.0 * math.pi * f1 * t) * 0.42
            + math.sin(2.0 * math.pi * f2 * t) * 0.58
        ) * e
        transient = 0.0
        if i < 220:
            transient = (rng.random() * 2.0 - 1.0) * 0.55 * (1.0 - i / 220.0)
        s = (body + transient) * 0.82
        out.append((s, s))
    return out


def gen_ui() -> list[tuple[float, float]]:
    n = int(SR * 0.036)
    out: list[tuple[float, float]] = []
    f = 1650.0
    for i in range(n):
        e = env_exp(i, n, 18.0)
        # 起始一小段高频加重，更像「咔嗒」
        click = 1.15 if i < 28 else 1.0
        s = math.sin(2.0 * math.pi * f * (i / SR)) * 0.82 * e * click
        out.append((s, s))
    return out


def gen_win() -> list[tuple[float, float]]:
    """短琶音 C5–E5–G5"""
    freqs = [523.25, 659.25, 783.99]
    note_len = int(SR * 0.055)
    gap = int(SR * 0.012)
    out: list[tuple[float, float]] = []
    for f in freqs:
        for i in range(note_len):
            t = i / SR
            e = env_exp(i, note_len, 9.0)
            s = math.sin(2.0 * math.pi * f * t) * 0.62 * e
            out.append((s, s))
        for _ in range(gap):
            out.append((0.0, 0.0))
    return out


def gen_bump() -> list[tuple[float, float]]:
    n = int(SR * 0.085)
    out: list[tuple[float, float]] = []
    rng = random.Random(99)
    f = 95.0
    for i in range(n):
        t = i / SR
        e = env_exp(i, n, 7.5)
        thump = math.sin(2.0 * math.pi * f * t) * 0.75 * e
        rustle = (rng.random() * 2.0 - 1.0) * 0.08 * e
        s = (thump + rustle) * 0.88
        out.append((s, s))
    return out


def main() -> None:
    root = Path(__file__).resolve().parent
    specs = [
        ("sfx_shoot.wav", gen_shoot),
        ("sfx_hit.wav", gen_hit),
        ("sfx_ui.wav", gen_ui),
        ("sfx_win.wav", gen_win),
        ("sfx_bump.wav", gen_bump),
    ]
    for name, gen in specs:
        write_stereo_wav(root / name, gen())
        print("wrote", name)


if __name__ == "__main__":
    main()
