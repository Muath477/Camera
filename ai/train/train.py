"""Train a label classifier on top of the CLIP image embeddings.

The image tower stays frozen (it is the same model the embedder serves);
only a linear layer is trained, so this runs on a laptop CPU in minutes and
needs tens of images per label, not thousands. Typical uses: your own
categories ("staff uniform", "delivery", "empty shelf") or re-checking the
detector's person/car labels on your cameras.

Input:  the folder prepare.py wrote (dataset/train, dataset/val)
Output: models/probe.json  labels, weights, bias; the embedder serves it at
                           POST /classify when mounted at /models
        models/report.json validation accuracy, per-label recall and the
                           confusion matrix

    python train.py dataset models --epochs 300
"""

import argparse
import json
import sys
from pathlib import Path

import numpy as np
from PIL import Image
from sentence_transformers import SentenceTransformer

IMAGE_MODEL = "clip-ViT-B-32"


def load_split(model: SentenceTransformer, root: Path, labels: list[str]):
    xs, ys = [], []
    for i, label in enumerate(labels):
        files = sorted((root / label).glob("*.jpg"))
        if not files:
            continue
        imgs = [Image.open(f).convert("RGB") for f in files]
        emb = model.encode(imgs, batch_size=32, convert_to_numpy=True)
        emb /= np.linalg.norm(emb, axis=1, keepdims=True)
        xs.append(emb)
        ys += [i] * len(files)
    return np.concatenate(xs).astype(np.float32), np.asarray(ys)


def softmax(z: np.ndarray) -> np.ndarray:
    z = z - z.max(axis=1, keepdims=True)
    e = np.exp(z)
    return e / e.sum(axis=1, keepdims=True)


def train(x: np.ndarray, y: np.ndarray, k: int, epochs: int, lr: float, l2: float, seed: int):
    """Multinomial logistic regression by full-batch gradient descent."""
    rng = np.random.default_rng(seed)
    w = rng.normal(0, 0.01, (k, x.shape[1])).astype(np.float32)
    b = np.zeros(k, dtype=np.float32)
    onehot = np.eye(k, dtype=np.float32)[y]
    # CLIP vectors are unit length, so logits need a scale to get confident.
    scale = 30.0
    for _ in range(epochs):
        p = softmax(scale * (x @ w.T) + b)
        g = (p - onehot) / len(x)
        w -= lr * (scale * g.T @ x + l2 * w)
        b -= lr * g.sum(axis=0)
    return scale * w, b


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("dataset", type=Path)
    ap.add_argument("out", type=Path)
    ap.add_argument("--epochs", type=int, default=300)
    ap.add_argument("--lr", type=float, default=0.5)
    ap.add_argument("--l2", type=float, default=1e-3)
    ap.add_argument("--seed", type=int, default=7)
    a = ap.parse_args()

    labels = sorted(p.name for p in (a.dataset / "train").iterdir() if p.is_dir())
    if len(labels) < 2:
        print("need at least two labels in dataset/train", file=sys.stderr)
        return 1
    model = SentenceTransformer(IMAGE_MODEL, device="cpu")
    xtr, ytr = load_split(model, a.dataset / "train", labels)
    xva, yva = load_split(model, a.dataset / "val", labels)
    print(f"{len(labels)} labels, {len(ytr)} train and {len(yva)} validation images")

    w, b = train(xtr, ytr, len(labels), a.epochs, a.lr, a.l2, a.seed)

    pred = np.argmax(xva @ w.T + b, axis=1)
    k = len(labels)
    cm = np.zeros((k, k), dtype=int)
    for t, p in zip(yva, pred):
        cm[t, p] += 1
    acc = float((pred == yva).mean())
    recall = {labels[i]: round(float(cm[i, i] / max(1, cm[i].sum())), 3) for i in range(k)}
    train_acc = float((np.argmax(xtr @ w.T + b, axis=1) == ytr).mean())

    a.out.mkdir(parents=True, exist_ok=True)
    (a.out / "probe.json").write_text(
        json.dumps({"model": IMAGE_MODEL, "labels": labels, "weights": w.round(6).tolist(), "bias": b.round(6).tolist()}),
        encoding="utf-8",
    )
    report = {
        "labels": labels,
        "train_images": int(len(ytr)),
        "val_images": int(len(yva)),
        "train_accuracy": round(train_acc, 3),
        "val_accuracy": round(acc, 3),
        "val_recall": recall,
        "confusion_matrix": {"rows_true_cols_pred": labels, "counts": cm.tolist()},
    }
    (a.out / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
