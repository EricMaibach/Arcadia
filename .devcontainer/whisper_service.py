from fastapi import FastAPI, UploadFile
from faster_whisper import WhisperModel
import tempfile
import shutil
import uvicorn
import os

app = FastAPI()

# Load Whisper model on GPU (CUDA)
# You can change "base" → "small", "medium", "large-v2", etc.
print("Getting ready to load model", flush=True)
model = WhisperModel("base", device="cuda", compute_type="float16")
print("Loaded model", flush=True)

@app.post("/transcribe")
async def transcribe_audio(file: UploadFile):
    print("transcribe_audio model called", flush=True)
    with tempfile.NamedTemporaryFile(delete=False, suffix=".mp3") as tmp:
        shutil.copyfileobj(file.file, tmp)
        tmp_path = tmp.name

    print(f"Saved file: {tmp_path}, size={os.path.getsize(tmp_path)} bytes", flush=True)

    segments, info = model.transcribe(tmp_path)
    text = " ".join([seg.text for seg in segments])

    return {"text": text.strip(), "language": info.language}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8002)