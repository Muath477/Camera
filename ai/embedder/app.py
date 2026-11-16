"""Embedding service for search by description.

Text and images land in the same 512-dimension CLIP ViT-B/32 space:
images through the CLIP image tower, text through a multilingual text
encoder distilled into that space, so an Arabic query ("شخص يحمل صندوقاً")
finds images the same way an English one does.

POST /embed          {"text": "..."}         -> {"embedding": [...512]}
POST /embed-image    raw JPEG/PNG body       -> {"embedding": [...512]}
POST /classify       raw JPEG/PNG body       -> {"labels": [{"label", "score"}]}
                     (only when a trained probe is mounted, see ai/train)
GET  /healthz

Runs on CPU; models are baked into the image at build time so the service
starts without internet.
"""

import io
import json
import os
from functools import lru_cache

import numpy as np
from fastapi import FastAPI, HTTPException, Request
from PIL import Image
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

IMAGE_MODEL = os.environ.get("IMAGE_MODEL", "clip-ViT-B-32")
TEXT_MODEL = os.environ.get("TEXT_MODEL", "sentence-transformers/clip-ViT-B-32-multilingual-v1")
PROBE_PATH = os.environ.get("PROBE_PATH", "/models/probe.json")
MAX_IMAGE_BYTES = 8 << 20

app = FastAPI(title="camera-embedder")


@lru_cache(maxsize=1)
def image_model() -> SentenceTransformer:
    return SentenceTransformer(IMAGE_MODEL, device="cpu")


@lru_cache(maxsize=1)
def text_model() -> SentenceTransformer:
    return SentenceTransformer(TEXT_MODEL, device="cpu")


def unit(v: np.ndarray) -> list[float]:
    n = float(np.linalg.norm(v)) or 1.0
    return (v / n).astype(np.float32).tolist()


class TextIn(BaseModel):
    text: str


@app.get("/healthz")
def healthz() -> dict:
    return {"ok": True, "probe": probe() is not None}


@app.post("/embed")
def embed(body: TextIn) -> dict:
    text = body.text.strip()
    if not text or len(text) > 500:
        raise HTTPException(400, "text must be 1 to 500 characters")
    return {"embedding": unit(text_model().encode(text))}


async def read_image(request: Request) -> Image.Image:
    raw = await request.body()
    if not raw or len(raw) > MAX_IMAGE_BYTES:
        raise HTTPException(400, "send one image up to 8 MB as the request body")
    try:
        return Image.open(io.BytesIO(raw)).convert("RGB")
    except Exception as exc:  # unreadable bytes are the caller's problem
        raise HTTPException(400, f"not an image: {exc}") from exc


def embed_image_vec(img: Image.Image) -> np.ndarray:
    return np.asarray(image_model().encode(img), dtype=np.float32)


@app.post("/embed-image")
async def embed_image(request: Request) -> dict:
    return {"embedding": unit(embed_image_vec(await read_image(request)))}


@lru_cache(maxsize=1)
def probe():
    """The linear classifier trained by ai/train/train.py, if mounted."""
    if not os.path.exists(PROBE_PATH):
        return None
    with open(PROBE_PATH, encoding="utf-8") as f:
        p = json.load(f)
    return p["labels"], np.asarray(p["weights"], dtype=np.float32), np.asarray(p["bias"], dtype=np.float32)


@app.post("/classify")
async def classify(request: Request) -> dict:
    p = probe()
    if p is None:
        raise HTTPException(404, "no trained probe is mounted")
    labels, w, b = p
    v = embed_image_vec(await read_image(request))
    v = v / (np.linalg.norm(v) or 1.0)
    logits = w @ v + b
    probs = np.exp(logits - logits.max())
    probs /= probs.sum()
    order = np.argsort(-probs)
    return {"labels": [{"label": labels[i], "score": round(float(probs[i]), 4)} for i in order]}
