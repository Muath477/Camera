# CPU-only embedding service. The models are downloaded at build time so the
# container starts and runs with no internet access.
FROM python:3.12-slim

WORKDIR /app
RUN pip install --no-cache-dir torch==2.7.0 --index-url https://download.pytorch.org/whl/cpu
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

ENV HF_HOME=/app/hf
RUN python -c "from sentence_transformers import SentenceTransformer as S; S('clip-ViT-B-32'); S('sentence-transformers/clip-ViT-B-32-multilingual-v1')"
ENV HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1

COPY app.py .
RUN useradd -r -u 10001 embedder && chown -R embedder /app
USER embedder
EXPOSE 8000
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "1"]
