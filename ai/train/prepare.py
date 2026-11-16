"""Organize labeled images into a train/validation dataset.

Input:  a folder with one sub-folder per label, e.g.
            raw/person/*.jpg  raw/car/*.jpg  raw/staff_uniform/*.jpg
        (from export_events.py, your own photos, or both)
Output: dataset/train/<label>/...  dataset/val/<label>/...
        dataset/manifest.csv   one row per image: split, label, path, sha256
        dataset/stats.json     counts per label and split

Duplicates (same bytes) are dropped, unreadable files are skipped and
reported, images are resized so the long side is at most 640 px, and the
split is stratified so every label has validation images.

    python prepare.py raw dataset --val 0.2 --seed 7
"""

import argparse
import csv
import hashlib
import io
import json
import random
import shutil
import sys
from pathlib import Path

from PIL import Image

EXTS = {".jpg", ".jpeg", ".png", ".webp"}
MIN_PER_LABEL = 4


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("raw", type=Path)
    ap.add_argument("out", type=Path)
    ap.add_argument("--val", type=float, default=0.2)
    ap.add_argument("--seed", type=int, default=7)
    ap.add_argument("--max-side", type=int, default=640)
    a = ap.parse_args()

    if a.out.exists():
        shutil.rmtree(a.out)
    rng = random.Random(a.seed)
    seen: set[str] = set()
    rows, skipped, dupes = [], [], 0
    stats: dict[str, dict[str, int]] = {}

    labels = sorted(p.name for p in a.raw.iterdir() if p.is_dir())
    for label in labels:
        files = sorted(f for f in (a.raw / label).rglob("*") if f.suffix.lower() in EXTS)
        good = []
        for f in files:
            raw = f.read_bytes()
            digest = hashlib.sha256(raw).hexdigest()
            if digest in seen:
                dupes += 1
                continue
            try:
                img = Image.open(io.BytesIO(raw)).convert("RGB")
            except Exception as exc:  # report and keep going
                skipped.append(f"{f}: {exc}")
                continue
            seen.add(digest)
            good.append((digest, img))
        if len(good) < MIN_PER_LABEL:
            print(f"skip label {label!r}: {len(good)} images, need at least {MIN_PER_LABEL}", file=sys.stderr)
            continue
        rng.shuffle(good)
        n_val = max(1, round(len(good) * a.val))
        for i, (digest, img) in enumerate(good):
            split = "val" if i < n_val else "train"
            img.thumbnail((a.max_side, a.max_side))
            dst = a.out / split / label / f"{digest[:16]}.jpg"
            dst.parent.mkdir(parents=True, exist_ok=True)
            img.save(dst, "JPEG", quality=90)
            rows.append({"split": split, "label": label, "path": dst.relative_to(a.out).as_posix(), "sha256": digest})
            stats.setdefault(label, {"train": 0, "val": 0})[split] += 1

    if not rows:
        print("no usable labels; put images in raw/<label>/", file=sys.stderr)
        return 1
    a.out.mkdir(parents=True, exist_ok=True)
    with open(a.out / "manifest.csv", "w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["split", "label", "path", "sha256"])
        w.writeheader()
        w.writerows(rows)
    summary = {"labels": stats, "duplicates_dropped": dupes, "unreadable": skipped}
    (a.out / "stats.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
